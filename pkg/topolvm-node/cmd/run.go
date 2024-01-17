package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/viper"
	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	clientwrapper "github.com/topolvm/topolvm/client"
	"github.com/topolvm/topolvm/controllers"
	"github.com/topolvm/topolvm/driver"
	"github.com/topolvm/topolvm/lvmd"
	"github.com/topolvm/topolvm/lvmd/command"
	"github.com/topolvm/topolvm/lvmd/proto"
	"github.com/topolvm/topolvm/runners"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8stypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"
	//+kubebuilder:scaffold:imports
)

// constants for node k8s API use
const (
	// AgentNotReadyNodeTaintKey contains the key of taints to be removed on driver startup
	AgentNotReadyNodeTaintKey = "topolvm.io/agent-not-ready"
)

var (
	scheme      = runtime.NewScheme()
	setupLog    = ctrl.Log.WithName("setup")
	cfgFilePath string

	// taintRemovalBackoff is the exponential backoff configuration for node taint removal
	taintRemovalBackoff = wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   2,
		Steps:    10, // Max delay = 0.5 * 2^9 = ~4 minutes
	}
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(topolvmv1.AddToScheme(scheme))
	utilruntime.Must(topolvmlegacyv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func subMain(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	nodename := viper.GetString("nodename")
	if len(nodename) == 0 {
		return errors.New("node name is not given")
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&config.zapOpts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: config.metricsAddr,
		LeaderElection:     false,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}
	client := clientwrapper.NewWrappedClient(mgr.GetClient())
	apiReader := clientwrapper.NewWrappedReader(mgr.GetAPIReader(), mgr.GetClient().Scheme())

	var lvService proto.LVServiceClient
	var vgService proto.VGServiceClient
	var health grpc_health_v1.HealthClient

	if config.embedLvmd {
		command.Containerized = true
		if err := loadConfFile(ctx, cfgFilePath); err != nil {
			return err
		}
		dcm := lvmd.NewDeviceClassManager(config.lvmd.DeviceClasses)
		ocm := lvmd.NewLvcreateOptionClassManager(config.lvmd.LvcreateOptionClasses)
		lvService, vgService = lvmd.NewEmbeddedServiceClients(ctx, dcm, ocm)
	} else {
		dialer := &net.Dialer{}
		dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", a)
		}
		conn, err := grpc.Dial(config.lvmdSocket, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialFunc))
		if err != nil {
			return err
		}
		defer conn.Close()
		lvService, vgService = proto.NewLVServiceClient(conn), proto.NewVGServiceClient(conn)
		health = grpc_health_v1.NewHealthClient(conn)
	}

	lvcontroller := controllers.NewLogicalVolumeReconcilerWithServices(client, nodename, vgService, lvService)

	if err := lvcontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalVolume")
		return err
	}
	//+kubebuilder:scaffold:builder

	// Add health checker to manager
	checker := runners.NewChecker(checkFunc(health, apiReader), 1*time.Minute) // adjusted signature
	if err := mgr.Add(checker); err != nil {
		return err
	}

	// Add metrics exporter to manager.
	// Note that grpc.ClientConn can be shared with multiple stubs/services.
	// https://github.com/grpc/grpc-go/tree/master/examples/features/multiplex
	if err := mgr.Add(runners.NewMetricsExporter(vgService, client, nodename)); err != nil { // adjusted signature
		return err
	}

	// Add gRPC server to manager.
	if err := os.MkdirAll(driver.DeviceDirectory, 0755); err != nil {
		return err
	}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(ErrorLoggingInterceptor))
	csi.RegisterIdentityServer(grpcServer, driver.NewIdentityServer(checker.Ready))
	nodeServer, err := driver.NewNodeServer(nodename, vgService, lvService, mgr) // adjusted signature
	if err != nil {
		return err
	}
	csi.RegisterNodeServer(grpcServer, nodeServer)
	err = mgr.Add(runners.NewGRPCRunner(grpcServer, config.csiSocket, false))
	if err != nil {
		return err
	}

	c := make(chan os.Signal, 2)
	signal.Notify(c, []os.Signal{os.Interrupt, syscall.SIGTERM}...)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	setupLog.Info("Setup K8s Client")
	setupLog.Info("Clear Startup Taint")
	go removeTaintInBackground()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch

func checkFunc(health grpc_health_v1.HealthClient, r client.Reader) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if health != nil {
			res, err := health.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
			if err != nil {
				return err
			}
			if status := res.GetStatus(); status != grpc_health_v1.HealthCheckResponse_SERVING {
				return fmt.Errorf("lvmd does not working: %s", status.String())
			}
		}

		var drv storagev1.CSIDriver
		return r.Get(ctx, types.NamespacedName{Name: topolvm.GetPluginName()}, &drv)
	}
}

func ErrorLoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	resp, err = handler(ctx, req)
	if err != nil {
		ctrl.Log.Error(err, "error on grpc call", "method", info.FullMethod)
	}
	return resp, err
}

func loadConfFile(ctx context.Context, cfgFilePath string) error {
	b, err := os.ReadFile(cfgFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(b, &config.lvmd)
	if err != nil {
		return err
	}
	log.FromContext(ctx).Info("configuration file loaded",
		"device_classes", config.lvmd.DeviceClasses,
		"file_name", cfgFilePath,
	)
	return nil
}

// Struct for JSON patch operations
type JSONPatch struct {
	OP    string      `json:"op,omitempty"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value"`
}

func getKubeClient() (*kubernetes.Clientset, error) {
	var (
		config *rest.Config
		err    error
	)
	config, err = rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	setupLog.Info("Using kube config: %+v", config)
	// creates the clientset
	return kubernetes.NewForConfig(config)
}

// removeTaintInBackground is a goroutine that retries removeNotReadyTaint with exponential backoff
func removeTaintInBackground() {
	backoffErr := wait.ExponentialBackoff(taintRemovalBackoff, func() (bool, error) {
		err := removeNotReadyTaint()
		if err != nil {
			setupLog.Error(err, "Unexpected failure when attempting to remove node taint(s)")
			return false, err
		}
		return true, nil
	})

	if backoffErr != nil {
		setupLog.Error(backoffErr, "Retries exhausted, giving up attempting to remove node taint(s)")
	}
}

func removeNotReadyTaint() error {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		setupLog.Info("NODE_NAME missing, skipping taint removal")
		return nil
	}
	clientset, err := getKubeClient()
	if err != nil {
		setupLog.Info("Failed to setup k8s client")
		return nil //lint:ignore nilerr If there are no k8s credentials, treat that as a soft failure
	}

	node, err := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var taintsToKeep []corev1.Taint
	for _, taint := range node.Spec.Taints {
		if taint.Key != AgentNotReadyNodeTaintKey {
			taintsToKeep = append(taintsToKeep, taint)
		} else {
			setupLog.Info("Queued taint for removal", "key", taint.Key, "effect", taint.Effect)
		}
	}

	if len(taintsToKeep) == len(node.Spec.Taints) {
		setupLog.Info("No taints to remove on node, skipping taint removal")
		return nil
	}

	patchRemoveTaints := []JSONPatch{
		{
			OP:    "test",
			Path:  "/spec/taints",
			Value: node.Spec.Taints,
		},
		{
			OP:    "replace",
			Path:  "/spec/taints",
			Value: taintsToKeep,
		},
	}

	patch, err := json.Marshal(patchRemoveTaints)
	if err != nil {
		return err
	}

	_, err = clientset.CoreV1().Nodes().Patch(context.Background(), nodeName, k8stypes.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return err
	}
	setupLog.Info("Removed taint(s) from local node", "node", nodeName)
	return nil
}
