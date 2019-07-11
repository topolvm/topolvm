package cmd

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/csi"
	"github.com/cybozu-go/topolvm/driver"
	"github.com/cybozu-go/topolvm/driver/k8s"
	lvmd "github.com/cybozu-go/topolvm/pkg/lvmd/cmd"
	"github.com/cybozu-go/well"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

const (
	modeNode             = "node"
	modeController       = "controller"
	defaultCSISocketName = "/run/topolvm/csi-topolvm.sock"
	defaultNamespace     = topolvm.SystemNamespace
)

var config struct {
	nodeName       string
	csiSocketName  string
	lvmdSocketName string
	namespace      string
}

var rootCmd = &cobra.Command{
	Use:     "csi-topolvm",
	Version: topolvm.Version,
	Short:   "TopoLVM CSI Plugin",
	Long:    `TopoLVM CSI Plugin`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 || (args[0] != modeNode && args[0] != modeController) {
			return fmt.Errorf("requires operation mode: %s or %s", modeNode, modeController)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		err := well.LogConfig{}.Apply()
		if err != nil {
			return err
		}

		err = os.Remove(config.csiSocketName)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		lis, err := net.Listen("unix", config.csiSocketName)
		if err != nil {
			return err
		}
		grpcServer := grpc.NewServer()

		identityServer := driver.NewIdentityService()
		csi.RegisterIdentityServer(grpcServer, identityServer)

		mode := args[0]
		switch mode {
		case modeController:
			if config.namespace == "" {
				return fmt.Errorf("--namespace is required for controller")
			}
			s, err := k8s.NewLogicalVolumeService(config.namespace)
			if err != nil {
				return err
			}
			controllerServer := driver.NewControllerService(s)
			csi.RegisterControllerServer(grpcServer, controllerServer)
			log.Info("start csi-topolvm", map[string]interface{}{
				"mode":       mode,
				"csi_socket": config.csiSocketName,
			})
		case modeNode:
			dialer := &net.Dialer{}
			dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
				return dialer.DialContext(ctx, "unix", a)
			}
			conn, err := grpc.Dial(config.lvmdSocketName, grpc.WithInsecure(), grpc.WithContextDialer(dialFunc))
			if err != nil {
				return err
			}
			defer conn.Close()

			err = os.MkdirAll(driver.DeviceDirectory, 0755)
			if err != nil {
				return err
			}

			if config.nodeName == "" {
				return fmt.Errorf("--node-name is required")
			}
			nodeServer := driver.NewNodeService(config.nodeName, conn)
			csi.RegisterNodeServer(grpcServer, nodeServer)
			log.Info("start csi-topolvm", map[string]interface{}{
				"mode":        mode,
				"csi_socket":  config.csiSocketName,
				"lvmd_socket": config.lvmdSocketName,
			})
		}

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

func init() {
	rootCmd.Flags().StringVar(&config.nodeName, "node-name", "", "The name of the node hosting csi-topolvm node service")
	rootCmd.Flags().StringVar(&config.csiSocketName, "csi-socket-name", defaultCSISocketName, "The socket name for CSI gRPC server")
	rootCmd.Flags().StringVar(&config.lvmdSocketName, "lvmd-socket-name", lvmd.DefaultSocketName, "The socket name for LVMD gRPC server, for node mode")
	rootCmd.Flags().StringVar(&config.namespace, "namespace", defaultNamespace, "Namespace for LogicalVolume CRD, for controller mode")
}
