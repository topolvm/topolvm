package cmd

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/hook"
	"github.com/spf13/cobra"
	"k8s.io/klog"
)

var config struct {
	metricsAddr string
	webhookAddr string
	certDir     string
	development bool
}

var rootCmd = &cobra.Command{
	Use:     "topolvm-hook",
	Version: topolvm.Version,
	Short:   "a webhook to mutate pods for TopoLVM",
	Long: `topolvm-hook is a mutating webhook server for TopoLVM.
It mutates pods with PersistentVolumeClaims to claim the storage
capacity in spec.resources of its first container.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain()
	},
}

func subMain() error {
	h, p, err := net.SplitHostPort(config.webhookAddr)
	if err != nil {
		return fmt.Errorf("invalid webhook addr: %v", err)
	}
	port, err := net.LookupPort("tcp", p)
	if err != nil {
		return fmt.Errorf("invalid webhook port: %v", err)
	}

	return hook.Run(nil, h, port, config.metricsAddr, config.certDir, config.development)
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
	fs.StringVar(&config.webhookAddr, "webhook-addr", ":8443", "Listen address for the webhook endpoint")
	fs.StringVar(&config.certDir, "cert-dir", "", "certificate directory")
	fs.BoolVar(&config.development, "development", false, "Use development logger config")

	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	fs.AddGoFlagSet(goflags)
}
