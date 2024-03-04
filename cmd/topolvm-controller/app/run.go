package app

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	clientwrapper "github.com/topolvm/topolvm/internal/client"
	"github.com/topolvm/topolvm/internal/controller"
	"github.com/topolvm/topolvm/internal/driver"
	"github.com/topolvm/topolvm/internal/hook"
	"github.com/topolvm/topolvm/internal/runners"
	"google.golang.org/grpc"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(topolvmv1.AddToScheme(scheme))
	utilruntime.Must(topolvmlegacyv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch

// Run builds and starts the manager with leader election.
func subMain() error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&config.zapOpts)))

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return err
	}

	hookHost, portStr, err := net.SplitHostPort(config.webhookAddr)
	if err != nil {
		return fmt.Errorf("invalid webhook addr: %v", err)
	}
	hookPort, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return fmt.Errorf("invalid webhook port: %v", err)
	}
	metricsServerOptions := metricsserver.Options{
		BindAddress: config.metricsAddr,
	}
	if config.secureMetricsServer {
		metricsServerOptions.SecureServing = true
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsServerOptions,
		HealthProbeBindAddress:  config.healthAddr,
		LeaderElection:          config.leaderElection,
		LeaderElectionID:        config.leaderElectionID,
		LeaderElectionNamespace: config.leaderElectionNamespace,
		RenewDeadline:           &config.leaderElectionRenewDeadline,
		RetryPeriod:             &config.leaderElectionRetryPeriod,
		LeaseDuration:           &config.leaderElectionLeaseDuration,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    hookHost,
			Port:    hookPort,
			CertDir: config.certDir,
		}),
	})
	if err != nil {
		return err
	}
	client := clientwrapper.NewWrappedClient(mgr.GetClient())
	apiReader := clientwrapper.NewWrappedReader(mgr.GetAPIReader(), mgr.GetClient().Scheme())

	if config.enableWebhooks {
		// register webhook handlers
		// admission.NewDecoder never returns non-nil error
		dec := admission.NewDecoder(scheme)
		wh := mgr.GetWebhookServer()
		wh.Register("/pod/mutate", hook.PodMutator(client, apiReader, dec))
		wh.Register("/pvc/mutate", hook.PVCMutator(client, apiReader, dec))
		if err := mgr.AddReadyzCheck("webhook", wh.StartedChecker()); err != nil {
			return err
		}
	}

	// register controllers
	nodecontroller := controller.NewNodeReconciler(client, config.skipNodeFinalize)
	if err := nodecontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		return err
	}

	pvccontroller := controller.NewPersistentVolumeClaimReconciler(client, apiReader)
	if err := pvccontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PersistentVolumeClaim")
		return err
	}

	//+kubebuilder:scaffold:builder

	// Add health checker to manager
	ctx := context.Background()
	check := func() error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		var drv storagev1.CSIDriver
		return apiReader.Get(ctx, types.NamespacedName{Name: topolvm.GetPluginName()}, &drv)
	}
	checker := runners.NewChecker(check, 1*time.Minute)
	if err := mgr.Add(checker); err != nil {
		return err
	}

	// Add gRPC server to manager.
	grpcServer := grpc.NewServer()
	csi.RegisterIdentityServer(grpcServer, driver.NewIdentityServer(checker.Ready))
	controllerSever, err := driver.NewControllerServer(mgr)
	if err != nil {
		return err
	}
	csi.RegisterControllerServer(grpcServer, controllerSever)

	// gRPC service itself should run even when the manager is *not* a leader
	// because CSI sidecar containers choose a leader.
	err = mgr.Add(runners.NewGRPCRunner(grpcServer, config.csiSocket, false))
	if err != nil {
		return err
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}
	return nil
}
