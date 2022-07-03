package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/topolvm/topolvm"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var config struct {
	metricsAddr      string
	healthAddr       string
	certDir          string
	leaderElectionID string
	zapOpts          zap.Options
}

var rootCmd = &cobra.Command{
	Use:     "topolvm-migrator-controller",
	Version: topolvm.Version,
	Short:   "TopoLVM CSI controller migrator",
	Long:    `topolvm-migrator-controller provides a function to migration from legacy to new resources for TopoLVM.`,
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
	fs.StringVar(&config.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	fs.StringVar(&config.healthAddr, "health-probe-bind-address", ":8081", "The TCP address that the controller should bind to for serving health probes.")
	fs.StringVar(&config.certDir, "cert-dir", "", "certificate directory")
	fs.StringVar(&config.leaderElectionID, "leader-election-id", "topolvm", "ID for leader election by controller-runtime")

	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	config.zapOpts.BindFlags(goflags)

	fs.AddGoFlagSet(goflags)
}
