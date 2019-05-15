package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/cybozu-go/topolvm/scheduler"
	"github.com/cybozu-go/well"
	"github.com/spf13/cobra"
)

var config struct {
	listenAddr string
}

var rootCmd = &cobra.Command{
	Use:   "topolvm-scheduler",
	Short: "A scheduler-extender for TopoLVM",
	Long:  `A scheduler-extender for TopoLVM`,
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

	serv := &well.HTTPServer{
		Server: &http.Server{
			Handler: scheduler.NewHandler(),
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
	rootCmd.Flags().StringVar(&config.listenAddr, "listen", "127.0.0.1:8000", "listen address")
}
