package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/cybozu-go/topolvm/lvmd"
	"github.com/cybozu-go/topolvm/lvmd/command"
	"github.com/cybozu-go/topolvm/lvmd/proto"
	"github.com/cybozu-go/well"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var config struct {
	vgName     string
	socketName string
}

// DefaultSocketName defines the default UNIX domain socket path.
const DefaultSocketName = "/run/topolvm/lvmd.sock"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "lvmd",
	Short: "a gRPC service to manage LVM volumes",
	Long:  `A gRPC service to manage LVM volumes.`,
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

	vg, err := command.FindVolumeGroup(config.vgName)
	if err != nil {
		return err
	}

	// UNIX domain socket file should be removed before listening.
	os.Remove(config.socketName)

	lis, err := net.Listen("unix", config.socketName)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	vgService, notifier := lvmd.NewVGService(vg)
	proto.RegisterVGServiceServer(grpcServer, vgService)
	proto.RegisterLVServiceServer(grpcServer, lvmd.NewLVService(vg, notifier))
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
	rootCmd.Flags().StringVar(&config.vgName, "volumegroup", "", "LVM volume group name")
	rootCmd.Flags().StringVar(&config.socketName, "listen", DefaultSocketName, "Unix domain socket name")
}
