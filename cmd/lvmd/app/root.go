package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/internal/lvmd"
	"github.com/topolvm/topolvm/internal/lvmd/command"
	"github.com/topolvm/topolvm/internal/profiling"
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
	cfgFilePath          string
	lvmPath              string
	zapOpts              zap.Options
	profilingBindAddress string
	metricsBindAddress   string
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

func subMain(parentCtx context.Context) error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))
	logger := log.FromContext(parentCtx)

	command.SetLVMPath(lvmPath)

	if err := loadConfFile(parentCtx, cfgFilePath); err != nil {
		return err
	}

	if err := lvmd.ValidateDeviceClasses(config.DeviceClasses); err != nil {
		return err
	}

	if config.LVMCommandPrefix != nil {
		if lvmPath != "" {
			return fmt.Errorf("cannot set both --lvm-path and lvm-command-prefix")
		}
		command.SetLVMCommandPrefix(config.LVMCommandPrefix)
	}

	vgs, err := command.ListVolumeGroups(parentCtx)
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
			_, err = vg.FindPool(parentCtx, dc.ThinPoolConfig.Name)
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

	ctx, stop := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	wg, pprofServer, metricsServer := startMetricsAndProfilingServers(logger)

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				if pprofServer != nil {
					if err := pprofServer.Shutdown(parentCtx); err != nil {
						logger.Error(err, "failed to shutdown pprof server")
					}
				}
				if metricsServer != nil {
					if err := metricsServer.Shutdown(parentCtx); err != nil {
						logger.Error(err, "failed to shutdown metrics server")
					}
				}
				grpcServer.GracefulStop()
				wg.Wait()
				return
			case <-ticker.C:
				notifier()
			}
		}
	}()

	return grpcServer.Serve(lis)
}

// startMetricsAndProfilingServers starts metrics and profiling servers if the bind addresses are set
// and returns a wait group to wait for the servers to stop.
func startMetricsAndProfilingServers(logger logr.Logger) (*sync.WaitGroup, *http.Server, *http.Server) {
	var wg sync.WaitGroup
	var pprofServer *http.Server
	if profilingBindAddress != "" {
		wg.Add(1)
		pprofServer = profiling.NewProfilingServer(profilingBindAddress)
		go func() {
			defer wg.Done()
			if err := pprofServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				logger.Error(err, "pprof server error")
			}
		}()
	}

	var metricsServer *http.Server
	if metricsBindAddress != "" {
		wg.Add(1)
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		metricsServer = &http.Server{
			Addr:    metricsBindAddress,
			Handler: mux,
		}
		go func() {
			defer wg.Done()
			if err := metricsServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				logger.Error(err, "metrics server error")
			}
		}()
	}

	return &wg, pprofServer, metricsServer
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
	fs := rootCmd.Flags()
	fs.StringVar(&cfgFilePath, "config", filepath.Join("/etc", "topolvm", "lvmd.yaml"), "config file")
	fs.StringVar(&lvmPath, "lvm-path", "", "lvm command path on the host OS. This is deprecated and users should use lvm-command-prefix setting instead.")
	fs.StringVar(&profilingBindAddress, "profiling-bind-address", "", "bind address to expose pprof profiling. If empty, profiling is disabled")
	fs.StringVar(&metricsBindAddress, "metrics-bind-address", ":8080", "bind address to expose prometheus metrics. If empty, metrics are disabled")

	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)
	fs.AddGoFlagSet(klogFlags)

	zapFlags := flag.NewFlagSet("zap", flag.ExitOnError)
	zapOpts.BindFlags(zapFlags)
	fs.AddGoFlagSet(zapFlags)
}
