package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/lvmetrics"
	lvmd "github.com/cybozu-go/topolvm/pkg/lvmd/cmd"
	"github.com/cybozu-go/well"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var config struct {
	socketName string
}

var rootCmd = &cobra.Command{
	Use:     "lvmetrics",
	Version: topolvm.Version,
	Short:   "annotate Node with LVM volume group metrics",
	Long: `Annotate Node resource with LVM volume group metrics.

This program should be run as a sidecar container in DaemonSet.
As this edits Node, the service account of the Pod should have
privilege to edit Node resources.`,
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

	nodeName := viper.GetString("nodename")
	if len(nodeName) == 0 {
		return errors.New("node name is not set")
	}
	patcher, err := lvmetrics.NewNodePatcher(nodeName)
	if err != nil {
		return err
	}

	dialer := &net.Dialer{}
	dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", a)
	}
	conn, err := grpc.Dial(config.socketName, grpc.WithInsecure(), grpc.WithContextDialer(dialFunc))
	if err != nil {
		return err
	}
	defer conn.Close()

	well.Go(func(ctx context.Context) error {
		return lvmetrics.WatchLVMd(ctx, conn, patcher)
	})
	well.Stop()
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
	rootCmd.Flags().StringVar(&config.socketName, "socket", lvmd.DefaultSocketName, "Unix domain socket name")
	rootCmd.Flags().String("nodename", "", "node resource name")
	viper.BindEnv("nodename", "NODE_NAME")
	viper.BindPFlag("nodename", rootCmd.Flags().Lookup("nodename"))
}
