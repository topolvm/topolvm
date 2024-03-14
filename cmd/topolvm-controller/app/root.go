package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/internal/driver"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const configName = "topolvm-controller-config"

var config struct {
	csiSocket                   string
	metricsAddr                 string
	secureMetricsServer         bool
	healthAddr                  string
	enableWebhooks              bool
	webhookAddr                 string
	certDir                     string
	leaderElection              bool
	leaderElectionID            string
	leaderElectionNamespace     string
	leaderElectionLeaseDuration time.Duration
	leaderElectionRenewDeadline time.Duration
	leaderElectionRetryPeriod   time.Duration
	skipNodeFinalize            bool
	zapOpts                     zap.Options
	controllerServerSettings    driver.ControllerServerSettings

	configFile string
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
	fs := rootCmd.Flags()
	// Command-Line Flags
	fs.StringVar(&config.csiSocket, "csi-socket", topolvm.DefaultCSISocket, "UNIX domain socket filename for CSI")
	fs.StringVar(&config.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	fs.BoolVar(&config.secureMetricsServer, "secure-metrics-server", false, "Secures the metrics server")
	fs.StringVar(&config.healthAddr, "health-probe-bind-address", ":8081", "The TCP address that the controller should bind to for serving health probes.")
	fs.StringVar(&config.webhookAddr, "webhook-addr", ":9443", "Listen address for the webhook endpoint")
	fs.BoolVar(&config.enableWebhooks, "enable-webhooks", true, "Enable webhooks")
	fs.StringVar(&config.certDir, "cert-dir", "", "certificate directory")
	fs.BoolVar(&config.leaderElection, "leader-election", true, "Enables leader election. This field is required to be set to true if concurrency is greater than 1 at any given point in time during rollouts.")
	fs.StringVar(&config.leaderElectionID, "leader-election-id", "topolvm", "ID for leader election by controller-runtime")
	fs.StringVar(&config.leaderElectionNamespace, "leader-election-namespace", "", "Namespace where the leader election resource lives. Defaults to the pod namespace if not set.")
	fs.DurationVar(&config.leaderElectionLeaseDuration, "leader-election-lease-duration", 15*time.Second, "Duration that non-leader candidates will wait to force acquire leadership. This is measured against time of last observed ack.")
	fs.DurationVar(&config.leaderElectionRenewDeadline, "leader-election-renew-deadline", 10*time.Second, "Duration that the acting controlplane will retry refreshing leadership before giving up. This is measured against time of last observed ack.")
	fs.DurationVar(&config.leaderElectionRetryPeriod, "leader-election-retry-period", 2*time.Second, "Duration the LeaderElector clients should wait between tries of actions.")
	fs.BoolVar(&config.skipNodeFinalize, "skip-node-finalize", false, "skips automatic cleanup of PhysicalVolumeClaims when a Node is deleted")

	// Special Binding for Config File, this will change the file that is read.
	fs.StringVar(&config.configFile, configName, fmt.Sprintf("%s.yaml", configName), "the file containing controller configuration settings. It can be in any format supported by viper (json, toml, yaml, hcl, ini, envfile). The default is yaml. The file can be located in the working directory, or in /etc/topolvm/")

	// klog flag bindings
	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	// zap flag bindings
	config.zapOpts.BindFlags(goflags)
	fs.AddGoFlagSet(goflags)

	// before we load the app, we need to load the config file into the flag set
	// this will default the flags to the config file, and then the command line flags will override, if set.
	rootCmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		if err := loadConfigFileIntoFlagSet(fs); err != nil {
			return err
		}

		// Controller Server Settings
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.TextUnmarshallerHookFunc(),
			Result:     &config.controllerServerSettings,
		})
		if err != nil {
			return err
		}

		// Decode the controller server settings, if they are set in the config file.
		// If they are not set, the default zero-value will be used.
		if err := decoder.Decode(viper.Get("controller-server-settings")); err != nil {
			return err
		}
		return nil
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// loadConfigFileIntoFlagSet loads the config file into the flag set and returns an error if it fails
// This function does not error if the config file is not found, as it is not required.
// Any value that is set in the flag set will override the config file.
// The config file can be located in the current working directory, or in /etc/topolvm/
// The config file can be any format that is supported by viper (json, toml, yaml, hcl, ini, envfile),
// but the default is yaml.
func loadConfigFileIntoFlagSet(fs *pflag.FlagSet) error {
	// Flags readable from config file and environment variables
	var errs []error
	fs.VisitAll(func(f *pflag.Flag) {
		// Skip the special config file flag
		if f.Name == configName {
			return
		}
		// Bind the flag to the config file
		if err := viper.BindPFlag(f.Name, f); err != nil {
			errs = append(errs, err)
		}
	})

	viper.AddConfigPath("/etc/topolvm")
	viper.AddConfigPath(".")

	configSplit := strings.Split(config.configFile, ".")
	name := strings.Join(configSplit[0:len(configSplit)-1], ".")
	fileType := configSplit[len(configSplit)-1]
	viper.SetConfigName(name)
	viper.SetConfigType(fileType)

	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return fmt.Errorf("fatal error config file: %w", err)
		}
	}
	return nil
}
