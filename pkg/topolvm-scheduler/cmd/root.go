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
	divisor    float64
}

var rootCmd = &cobra.Command{
	Use:   "topolvm-scheduler",
	Short: "a scheduler-extender for TopoLVM",
	Long: `A scheduler-extender for TopoLVM.

The extender implements filter and prioritize verbs.

The filter verb is "predicate" and served at "/predicate" via HTTP.
It filters out nodes that have less storage capacity than requested.
The requested capacity is read from "topolvm.cybozu.com/capacity"
resource value.

The prioritize verb is "prioritize" and served at "/prioritize" via HTTP.
It scores nodes with this formula:

    min(10, max(0, log2(capacity >> 30 / divisor)))

The default divisor is 1.  It can be changed with a command-line option.
`,
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

	h, err := scheduler.NewHandler(config.divisor)
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
	rootCmd.Flags().Float64Var(&config.divisor, "divisor", 1, "capacity divisor")
}
