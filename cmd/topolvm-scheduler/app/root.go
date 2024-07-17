package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/internal/profiling"
	"github.com/topolvm/topolvm/internal/scheduler"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"
)

var cfgFilePath string
var zapOpts zap.Options

const defaultDivisor = 1
const defaultListenAddr = ":8000"

// Config represents configuration parameters for topolvm-scheduler
type Config struct {
	// ListenAddr is listen address of topolvm-scheduler.
	ListenAddr string `json:"listen"`
	// Divisors is a mapping between device-class names and their divisors.
	Divisors map[string]float64 `json:"divisors"`
	// DefaultDivisor is the default divisor value.
	DefaultDivisor float64 `json:"default-divisor"`
	// ProfilingBindAddress is the bind address to expose pprof profiling. If empty, profiling is disabled.
	ProfilingBindAddress string `json:"profiling-bind-address"`
}

var config = &Config{
	ListenAddr:     defaultListenAddr,
	DefaultDivisor: defaultDivisor,
}

var rootCmd = &cobra.Command{
	Use:     "topolvm-scheduler",
	Version: topolvm.Version,
	Short:   "a scheduler-extender for TopoLVM",
	Long: `A scheduler-extender for TopoLVM.

The extender implements filter and prioritize verbs.

The filter verb is "predicate" and served at "/predicate" via HTTP.
It filters out nodes that have less storage capacity than requested.
The requested capacity is read from "capacity.topolvm.io/<device-class>"
resource value.

The prioritize verb is "prioritize" and served at "/prioritize" via HTTP.
It scores nodes with this formula:

    min(10, max(0, log2(capacity >> 30 / divisor)))

The default divisor is 1.  It can be changed with a command-line option.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain(cmd.Context())
	},
}

func subMain(parentCtx context.Context) error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))
	logger := log.FromContext(parentCtx)

	if len(cfgFilePath) != 0 {
		b, err := os.ReadFile(cfgFilePath)
		if err != nil {
			return err
		}
		err = yaml.Unmarshal(b, config)
		if err != nil {
			return err
		}
	}

	h, err := scheduler.NewHandler(config.DefaultDivisor, config.Divisors)
	if err != nil {
		return err
	}

	serv := &http.Server{
		Addr:        config.ListenAddr,
		Handler:     accessLogHandler(parentCtx, h),
		ReadTimeout: 30 * time.Second,
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, stop := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	defer stop() // stop() should be called before wg.Wait() to stop the goroutine correctly.

	var pprofServer *http.Server
	if config.ProfilingBindAddress != "" {
		pprofServer = profiling.NewProfilingServer(config.ProfilingBindAddress)
		go func() {
			if err := pprofServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				logger.Error(err, "pprof server error")
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		if pprofServer != nil {
			if err := pprofServer.Shutdown(parentCtx); err != nil {
				logger.Error(err, "failed to shutdown pprof server")
			}
		}
		if err := serv.Shutdown(parentCtx); err != nil {
			logger.Error(err, "failed to shutdown gracefully")
		}
	}()

	err = serv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
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
	rootCmd.PersistentFlags().StringVar(&cfgFilePath, "config", "", "config file")
}
