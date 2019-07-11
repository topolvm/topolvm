package cmd

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	topolvmv1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	"github.com/cybozu-go/topolvm/topolvm-node/controllers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	topolvmv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme

	fs := rootCmd.Flags()
	fs.String("metrics-addr", ":28080", "Bind address for the metrics endpoint")
	fs.String("lvmd-socket", "/run/topolvm/lvmd.sock", "UNIX domain socket of lvmd service")
	fs.String("node-name", "", "The name of the node hosting topolvm-node service")
	fs.Bool("development", false, "Use development logger config")

	if err := viper.BindPFlags(fs); err != nil {
		panic(err)
	}
	if err := cobra.MarkFlagRequired(fs, "node-name"); err != nil {
		panic(err)
	}
	viper.SetEnvPrefix("topo")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	fs.AddGoFlagSet(goflags)
}

var rootCmd = &cobra.Command{
	Use:   "topolvm-node",
	Short: "",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain()
	},
}

func subMain() error {
	ctrl.SetLogger(zap.Logger(viper.GetBool("development")))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme, MetricsBindAddress: viper.GetString("metrics-addr")})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	dialer := &net.Dialer{}
	dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", a)
	}
	conn, err := grpc.Dial(viper.GetString("lvmd-socket"), grpc.WithInsecure(), grpc.WithContextDialer(dialFunc))
	if err != nil {
		return err
	}
	defer conn.Close()

	err = controllers.NewLogicalVolumeReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("LogicalVolume"),
		viper.GetString("node-name"),
		conn,
	).SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalVolume")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	return nil
}
