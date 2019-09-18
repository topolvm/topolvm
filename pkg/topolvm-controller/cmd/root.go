package cmd

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/controller"
	"github.com/spf13/cobra"
	"k8s.io/klog"
)

var config struct {
	metricsAddr     string
	stalePeriod     time.Duration
	cleanupInterval time.Duration
	development     bool
}

var rootCmd = &cobra.Command{
	Use:     "topolvm-controller",
	Version: topolvm.Version,
	Short:   "a custom controller for TopoLVM",
	Long: `topolvm-controller is a custom controller for TopoLVM.
It runs finalizer for Node and cleans up stale resources.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain()
	},
}

func subMain() error {
	return controller.Run(nil, config.metricsAddr, config.stalePeriod, config.cleanupInterval, config.development)
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
	fs.StringVar(&config.metricsAddr, "metrics-addr", ":8080", "Listen address for metrics")
	fs.DurationVar(&config.stalePeriod, "stale-period", 24*time.Hour, "LogicalVolume is cleaned up if it is not deleted within this period")
	fs.DurationVar(&config.cleanupInterval, "cleanup-interval", 10*time.Minute, "Cleaning up interval for LogicalVolume")
	fs.BoolVar(&config.development, "development", false, "Use development logger config")

	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	fs.AddGoFlagSet(goflags)
}
