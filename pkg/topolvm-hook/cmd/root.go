package cmd

import (
	"fmt"
	"net"
	"os"

	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/hook"
	"github.com/spf13/cobra"
)

var config struct {
	metricsAddr string
	webhookAddr string
	certDir     string
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

	return hook.Run(h, port, config.metricsAddr, config.certDir)
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
	rootCmd.Flags().StringVar(&config.metricsAddr, "metrics-addr", ":8080", "Listen address for metrics")
	rootCmd.Flags().StringVar(&config.webhookAddr, "webhook-addr", ":8443", "Listen address for the webhook endpoint")
	rootCmd.Flags().StringVar(&config.certDir, "cert-dir", "", "certificate directory")
}
