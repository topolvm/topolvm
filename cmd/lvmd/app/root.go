package app

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/internal/lvmd"
	"github.com/topolvm/topolvm/internal/lvmd/command"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	lvmdTypes "github.com/topolvm/topolvm/pkg/lvmd/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	cfgFilePath string
	lvmPath     string
	zapOpts     zap.Options
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "lvmd",
	Version: topolvm.Version,
	Short:   "a gRPC service to manage LVM volumes",
	Long: `A gRPC service to manage LVM volumes.

lvmd handles a LVM volume group and provides gRPC API to manage logical
volumes in the volume group.

If command-line option "spare" is not zero, that value multiplied by 1 GiB
will be subtracted from the value lvmd reports as the free space of the
volume group.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain(cmd.Context())
	},
}

func subMain(ctx context.Context) error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))
	logger := log.FromContext(ctx)

	command.SetLVMPath(lvmPath)

	if err := loadConfFile(ctx, cfgFilePath); err != nil {
		return err
	}

	if err := lvmd.ValidateDeviceClasses(config.DeviceClasses); err != nil {
		return err
	}

	vgs, err := command.ListVolumeGroups(ctx)
	if err != nil {
		logger.Error(err, "error while retrieving volume groups")
		return err
	}

	for _, dc := range config.DeviceClasses {
		vg, err := command.SearchVolumeGroupList(vgs, dc.VolumeGroup)
		if err != nil {
			logger.Error(err, "volume group not found", "volume_group", dc.VolumeGroup)
			return err
		}

		if dc.Type == lvmdTypes.TypeThin {
			_, err = vg.FindPool(ctx, dc.ThinPoolConfig.Name)
			if err != nil {
				logger.Error(err, "Thin pool not found:", "thinpool", dc.ThinPoolConfig.Name)
				return err
			}
		}
	}

	// UNIX domain socket file should be removed before listening.
	err = os.Remove(config.SocketName)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	lis, err := net.Listen("unix", config.SocketName)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	dcm := lvmd.NewDeviceClassManager(config.DeviceClasses)
	ocm := lvmd.NewLvcreateOptionClassManager(config.LvcreateOptionClasses)
	vgService, notifier := lvmd.NewVGService(dcm)
	proto.RegisterVGServiceServer(grpcServer, vgService)
	proto.RegisterLVServiceServer(grpcServer, lvmd.NewLVService(dcm, ocm, notifier))
	grpc_health_v1.RegisterHealthServer(grpcServer, lvmd.NewHealthService())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				grpcServer.GracefulStop()
				return
			case <-ticker.C:
				notifier()
			}
		}
	}()

	return grpcServer.Serve(lis)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

//nolint:lll
func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFilePath, "config", filepath.Join("/etc", "topolvm", "lvmd.yaml"), "config file")
	rootCmd.PersistentFlags().StringVar(&lvmPath, "lvm-path", "", "lvm command path on the host OS")

	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	zapOpts.BindFlags(goflags)
}
