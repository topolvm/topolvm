package cmd

import (
	"fmt"
	"net"
	"os"

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

const defaultSocketName = "/run/topolvm/lvmd.sock"

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

	lis, err := net.Listen("unix", config.socketName)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	proto.RegisterLVServiceServer(grpcServer, lvmd.NewLVService(vg))
	proto.RegisterVGServiceServer(grpcServer, lvmd.NewVGService(vg))
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

func init() {
	rootCmd.Flags().StringVar(&config.vgName, "volumegroup", "", "LVM volume group name")
	rootCmd.Flags().StringVar(&config.socketName, "listen", defaultSocketName, "Unix domain socket name")
}
