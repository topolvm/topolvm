package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/scheduler"
	"github.com/cybozu-go/well"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var cfgFilePath string

// Config represents configuration parameters for topolvm-scheduler
type Config struct {
	// ListenAddr is listen address of topolvm-scheduler
	ListenAddr string `json:"listen"`
	// Divisors are capacity divisor for each volume group
	Divisors map[string]float64 `json:"divisors"`
	// DefaultDivisor is the default value of divisor which is used if the value for the given volume group does not exist in `divisors`
	DefaultDivisor float64 `json:"default-divisor"`
}

var config Config

var rootCmd = &cobra.Command{
	Use:     "topolvm-scheduler",
	Version: topolvm.Version,
	Short:   "a scheduler-extender for TopoLVM",
	Long: `A scheduler-extender for TopoLVM.

The extender implements filter and prioritize verbs.

The filter verb is "predicate" and served at "/predicate" via HTTP.
It filters out nodes that have less storage capacity than requested.
The requested capacity is read from "capacity.topolvm.io/<volume group name>"
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

	b, err := ioutil.ReadFile(cfgFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(b, config)
	if err != nil {
		return err
	}

	h, err := scheduler.NewHandler(config.DefaultDivisor, config.Divisors)
	if err != nil {
		return err
	}

	serv := &well.HTTPServer{
		Server: &http.Server{
			Addr:    config.ListenAddr,
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
	rootCmd.Flags().StringVar(&config.ListenAddr, "listen", ":8000", "listen address")
	rootCmd.PersistentFlags().StringVar(&cfgFilePath, "config", filepath.Join("etc", "topolvm-scheduler", "config.yml"), "config file")
}
