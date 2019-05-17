package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/cybozu-go/topolvm/hook"
	"github.com/cybozu-go/well"
	"github.com/spf13/cobra"
)

var config struct {
	listenAddr string
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "topolvm-hook",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
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

	h, err := hook.NewHandler()
	if err != nil {
		return err
	}

	serv := &well.HTTPServer{
		Server: &http.Server{
			Addr:    config.listenAddr,
			Handler: h,
		},
	}

	err = serv.ListenAndServe()
	if err != nil {
		return err
	}
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
	rootCmd.Flags().StringVar(&config.listenAddr, "listen", ":8000", "listen address")
}
