package app

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/topolvm/topolvm"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Options contains the command's deliberately narrow static configuration.
type Options struct {
	ConfigFile              string
	LeaderElect             bool
	LeaderElectionNamespace string
	LeaderElectionID        string
	MetricsBindAddress      string
	HealthProbeBindAddress  string
	SyncPeriod              time.Duration
	Zap                     zap.Options
}

// NewCommand constructs a fresh command, which also makes flag validation easy
// to exercise without global Cobra state.
func NewCommand() *cobra.Command {
	opts := Options{}
	cmd := &cobra.Command{
		Use:           "topolvm-capacity-template-controller",
		Version:       topolvm.Version,
		Short:         "Publish TopoLVM capacity for Datadog NodeGroup templates",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			if err := ValidateOptions(opts); err != nil {
				return err
			}
			return run(opts)
		},
	}

	fs := cmd.Flags()
	fs.StringVar(&opts.ConfigFile, "config", "/etc/topolvm/capacity-template.yaml", "Path to capacity-template config")
	fs.BoolVar(&opts.LeaderElect, "leader-elect", true, "Enable leader election")
	fs.StringVar(
		&opts.LeaderElectionNamespace,
		"leader-election-namespace",
		"kube-system",
		"Namespace for the leader-election Lease",
	)
	fs.StringVar(
		&opts.LeaderElectionID,
		"leader-election-id",
		"datadog-topolvm-capacity-template-controller",
		"Leader-election Lease name",
	)
	fs.StringVar(&opts.MetricsBindAddress, "metrics-bind-address", ":8082", "Address for the metrics endpoint")
	fs.StringVar(&opts.HealthProbeBindAddress, "health-probe-bind-address", ":8083", "Address for health probes")
	fs.DurationVar(&opts.SyncPeriod, "sync-period", 5*time.Minute, "Maximum convergence period for capacities")

	goFlags := flag.NewFlagSet("klog", flag.ContinueOnError)
	klog.InitFlags(goFlags)
	opts.Zap.BindFlags(goFlags)
	fs.AddGoFlagSet(goFlags)
	return cmd
}

// ValidateOptions rejects unsafe or malformed static configuration before a
// Kubernetes client is created.
func ValidateOptions(opts Options) error {
	if opts.ConfigFile == "" {
		return fmt.Errorf("--config must not be empty")
	}
	if errs := validation.IsDNS1123Label(opts.LeaderElectionNamespace); len(errs) != 0 {
		return fmt.Errorf("invalid --leader-election-namespace %q: %v", opts.LeaderElectionNamespace, errs)
	}
	if errs := validation.IsDNS1123Subdomain(opts.LeaderElectionID); len(errs) != 0 {
		return fmt.Errorf("invalid --leader-election-id %q: %v", opts.LeaderElectionID, errs)
	}
	if err := validateBindAddress(opts.MetricsBindAddress); err != nil {
		return fmt.Errorf("invalid --metrics-bind-address: %w", err)
	}
	if err := validateBindAddress(opts.HealthProbeBindAddress); err != nil {
		return fmt.Errorf("invalid --health-probe-bind-address: %w", err)
	}
	if opts.SyncPeriod <= 0 {
		return fmt.Errorf("--sync-period must be positive")
	}
	return nil
}

func validateBindAddress(address string) error {
	if address == "" {
		return fmt.Errorf("address must not be empty")
	}
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if port == "" {
		return fmt.Errorf("port must not be empty")
	}
	if _, err := net.LookupPort("tcp", port); err != nil {
		return err
	}
	return nil
}

// Execute runs the standalone capacity-template controller command.
func Execute() {
	if err := NewCommand().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
