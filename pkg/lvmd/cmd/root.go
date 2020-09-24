package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/well"
	"github.com/spf13/cobra"
	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/lvmd"
	"github.com/topolvm/topolvm/lvmd/command"
	"github.com/topolvm/topolvm/lvmd/proto"
	"google.golang.org/grpc"
	"sigs.k8s.io/yaml"
)

var cfgFilePath string

// Config represents configuration parameters for lvmd
type Config struct {
	// SocketName is Unix domain socket name
	SocketName string `json:"socket-name"`
	// DeviceClasses is
	DeviceClasses []*lvmd.DeviceClass `json:"device-classes"`
}

var config = &Config{
	SocketName: topolvm.DefaultLVMdSocket,
}

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
		return subMain()
	},
}

func subMain() error {
	err := well.LogConfig{}.Apply()
	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile(cfgFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(b, &config)
	if err != nil {
		return err
	}
	log.Info("configuration file loaded: ", map[string]interface{}{
		"device_classes": config.DeviceClasses,
		"socket_name":    config.SocketName,
		"file_name":      cfgFilePath,
	})
	err = lvmd.ValidateDeviceClasses(config.DeviceClasses)
	if err != nil {
		return err
	}
	for _, dc := range config.DeviceClasses {
		_, err := command.FindVolumeGroup(dc.VolumeGroup)
		if err != nil {
			return err
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
	manager := lvmd.NewDeviceClassManager(config.DeviceClasses)
	vgService, notifier := lvmd.NewVGService(manager)
	proto.RegisterVGServiceServer(grpcServer, vgService)
	proto.RegisterLVServiceServer(grpcServer, lvmd.NewLVService(manager, notifier))
	well.Go(func(ctx context.Context) error {
		return grpcServer.Serve(lis)
	})
	well.Go(func(ctx context.Context) error {
		<-ctx.Done()
		grpcServer.GracefulStop()
		return nil
	})
	well.Go(func(ctx context.Context) error {
		ticker := time.NewTicker(10 * time.Minute)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return nil
			case <-ticker.C:
				notifier()
			}
		}
	})
	err = well.Wait()
	if err != nil && !well.IsSignaled(err) {
		return err
	}
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFilePath, "config", filepath.Join("/etc", "topolvm", "lvmd.yaml"), "config file")
}
