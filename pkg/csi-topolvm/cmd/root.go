package cmd

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/cybozu-go/topolvm/csi"
	"github.com/cybozu-go/well"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

const socketName = "/run/topolvm/csi-topolvm.sock"

var rootCmd = &cobra.Command{
	Use:   "csi-topolvm",
	Short: "TopoLVM CSI Plugin",
	Long:  `TopoLVM CSI Plugin`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := well.LogConfig{}.Apply()
		if err != nil {
			return err
		}

		os.Remove(socketName)
		lis, err := net.Listen("unix", socketName)
		if err != nil {
			return err
		}
		grpcServer := grpc.NewServer()

		identityServer := csi.NewIdentityService()
		csi.RegisterIdentityServer(grpcServer, identityServer)
		controllerServer := csi.NewControllerService()
		csi.RegisterControllerServer(grpcServer, controllerServer)
		nodeServer := csi.NewNodeService()
		csi.RegisterNodeServer(grpcServer, nodeServer)

		well.Go(func(ctx context.Context) error {
			return grpcServer.Serve(lis)
		})
		well.Go(func(ctx context.Context) error {
			<-ctx.Done()
			grpcServer.GracefulStop()
			return nil
		})

		err = well.Wait()
		if err != nil && !well.IsSignaled(err) {
			return err
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
