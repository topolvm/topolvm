package app

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/pkg/driver"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	// DefaultMinimumAllocationSizeBlock is the default minimum size for a block volume.
	// Derived from the usual physical extent size of 4Mi * 2 (for accommodating metadata)
	DefaultMinimumAllocationSizeBlock = "8Mi"
	// DefaultMinimumAllocationSizeXFS is the default minimum size for a filesystem volume with XFS formatting.
	// Derived from the hard XFS minimum size of 300Mi that is enforced by the XFS filesystem.
	DefaultMinimumAllocationSizeXFS = "300Mi"
	// DefaultMinimumAllocationSizeExt4 is the default minimum size for a filesystem volume with ext4 formatting.
	// Derived from the usual 4096K blocks, 1024 inode default and journaling overhead,
	// Allows for more than 80% free space after formatting, anything lower significantly reduces this percentage.
	DefaultMinimumAllocationSizeExt4 = "32Mi"
	// DefaultMinimumAllocationSizeBtrfs is the default minimum size for a filesystem volume with btrfs formatting.
	// Btrfs changes its minimum allocation size based on various underlying device block settings and the host OS,
	// but 200Mi seemed to be safe after some experimentation.
	DefaultMinimumAllocationSizeBtrfs = "200Mi"
)

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
	profilingBindAddress        string
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

//nolint:lll
func init() {
	fs := rootCmd.Flags()
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
	fs.StringVar(&config.profilingBindAddress, "profiling-bind-address", "", "Bind pprof profiling to the given network address. If empty, profiling is disabled.")

	driver.QuantityVar(fs, &config.controllerServerSettings.MinimumAllocationSettings.Block,
		"minimum-allocation-block",
		resource.MustParse(DefaultMinimumAllocationSizeBlock),
		"Minimum Allocation Sizing for block storage. Logical Volumes will always be at least this big.")
	config.controllerServerSettings.MinimumAllocationSettings.Filesystem = make(map[string]driver.Quantity)
	for filesystem, minimum := range map[string]resource.Quantity{
		"ext4":  resource.MustParse(DefaultMinimumAllocationSizeExt4),
		"xfs":   resource.MustParse(DefaultMinimumAllocationSizeXFS),
		"btrfs": resource.MustParse(DefaultMinimumAllocationSizeBtrfs),
	} {
		config.controllerServerSettings.MinimumAllocationSettings.Filesystem[filesystem] = driver.NewQuantityFlagVar(fs,
			fmt.Sprintf("minimum-allocation-%s", filesystem),
			minimum,
			fmt.Sprintf("Minimum Allocation Sizing for volumes with the %s filesystem. Logical Volumes will always be at least this big.", filesystem))
	}

	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)
	fs.AddGoFlagSet(klogFlags)

	zapFlags := flag.NewFlagSet("zap", flag.ExitOnError)
	config.zapOpts.BindFlags(zapFlags)
	fs.AddGoFlagSet(zapFlags)
}
