package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/topolvm/topolvm"
	"k8s.io/klog"
)

var config struct {
	csiSocket        string
	metricsAddr      string
	webhookAddr      string
	certDir          string
	leaderElectionID string
	development      bool
}

var rootCmd = &cobra.Command{
	Use:     "topolvm-controller",
	Version: topolvm.Version,
	Short:   "TopoLVM CSI controller",
	Long: `topolvm-controller provides CSI controller service.
It also works as a custom Kubernetes controller.`,

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
	fs.StringVar(&config.metricsAddr, "metrics-addr", ":8080", "Listen address for metrics")
	fs.StringVar(&config.webhookAddr, "webhook-addr", ":8443", "Listen address for the webhook endpoint")
	fs.StringVar(&config.certDir, "cert-dir", "", "certificate directory")
	fs.StringVar(&config.leaderElectionID, "leader-election-id", "topolvm", "ID for leader election by controller-runtime")
	fs.BoolVar(&config.development, "development", false, "Use development logger config")

	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	fs.AddGoFlagSet(goflags)
}
