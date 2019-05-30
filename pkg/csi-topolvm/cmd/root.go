package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/csi"
	"github.com/cybozu-go/topolvm/csi/k8s"
	"github.com/cybozu-go/well"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const (
	modeNode          = "node"
	modeController    = "controller"
	defaultSocketName = "/run/topolvm/csi-topolvm.sock"
	defaultNamespace  = topolvm.SystemNamespace
)

var rootCmd = &cobra.Command{
	Use:   "csi-topolvm",
	Short: "TopoLVM CSI Plugin",
	Long:  `TopoLVM CSI Plugin`,
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

		socketName := viper.GetString("socket-name")
		os.Remove(socketName)
		lis, err := net.Listen("unix", socketName)
		if err != nil {
			return err
		}
		grpcServer := grpc.NewServer()

		identityServer := csi.NewIdentityService()
		csi.RegisterIdentityServer(grpcServer, identityServer)

		mode := args[0]
		switch mode {
		case modeController:
			namespace := viper.GetString("namespace")
			if namespace == "" {
				return fmt.Errorf("--namespace is required for controller")
			}
			s, err := k8s.NewLogicalVolumeService(namespace)
			if err != nil {
				return err
			}
			controllerServer := csi.NewControllerService(s)
			csi.RegisterControllerServer(grpcServer, controllerServer)
		case modeNode:
			nodeName := viper.GetString("node-name")
			if nodeName == "" {
				return fmt.Errorf("--node-name is required")
			}
			vgName := viper.GetString("volume-group")
			if vgName == "" {
				return fmt.Errorf("--volume-group is required")
			}
			nodeServer := csi.NewNodeService(nodeName, vgName)
			csi.RegisterNodeServer(grpcServer, nodeServer)
		}

		log.Info("start csi-topolvm", map[string]interface{}{
			"mode":   mode,
			"socket": socketName,
		})
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
	fs := rootCmd.Flags()
	fs.String("volume-group", "", "LVM volume group name")
	fs.String("node-name", "", "The name of the node hosting csi-topolvm node service")
	fs.String("socket-name", defaultSocketName, "The socket name for gRPC")
	fs.String("namespace", defaultNamespace, "Namespace for LogicalVolume CRD")

	if err := viper.BindPFlags(fs); err != nil {
		panic(err)
	}

	viper.SetEnvPrefix("topo")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}
