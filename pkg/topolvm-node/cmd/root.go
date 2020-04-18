package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/cybozu-go/topolvm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog"
)

var config struct {
	csiSocket   string
	lvmdSocket  string
	metricsAddr string
	defaultVG   string
	development bool
}

var rootCmd = &cobra.Command{
	Use:     "topolvm-node",
	Version: topolvm.Version,
	Short:   "TopoLVM CSI node",
	Long: `topolvm-node provides CSI node service.
It also works as a custom Kubernetes controller.

The node name where this program runs must be given by either
NODE_NAME environment variable or --nodename flag.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain()
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
	fs.StringVar(&config.csiSocket, "csi-socket", topolvm.DefaultCSISocket, "UNIX domain socket filename for CSI")
	fs.StringVar(&config.lvmdSocket, "lvmd-socket", topolvm.DefaultLVMdSocket, "UNIX domain socket of lvmd service")
	fs.StringVar(&config.metricsAddr, "metrics-addr", ":8080", "Listen address for metrics")
	fs.StringVar(&config.defaultVG, "default-vg", "", "Default Volume Group")
	fs.String("nodename", "", "The resource name of the running node")
	fs.BoolVar(&config.development, "development", false, "Use development logger config")

	viper.BindEnv("nodename", "NODE_NAME")
	viper.BindPFlag("nodename", fs.Lookup("nodename"))

	rootCmd.MarkFlagRequired("default-vg")

	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	fs.AddGoFlagSet(goflags)
}
