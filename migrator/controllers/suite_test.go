package controllers

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var testCtx, testCancel = context.WithCancel(context.Background())

func strPtr(s string) *string                                                  { return &s }
func volumeModePtr(m corev1.PersistentVolumeMode) *corev1.PersistentVolumeMode { return &m }

var PVCProtectionFinalizer = "kubernetes.io/pvc-protection"

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	SetDefaultEventuallyTimeout(time.Minute)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	By("start test manager")
	scheme := runtime.NewScheme()
	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).ToNot(HaveOccurred())
	err = topolvmlegacyv1.AddToScheme(scheme)
	Expect(err).ToNot(HaveOccurred())
	err = topolvmv1.AddToScheme(scheme)
	Expect(err).ToNot(HaveOccurred())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: ":8081",
	})
	Expect(err).ToNot(HaveOccurred())

	pvccontroller := &PersistentVolumeClaimReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
	}
	err = pvccontroller.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	nodecontroller := &NodeReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
	}
	err = nodecontroller.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	logicalvolumecontroller := &LogicalVolumeReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		NodeName:  testNodeForLogicalVolume,
	}
	err = logicalvolumecontroller.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	pvcontroller := &PersistentVolumeReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
	}
	err = pvcontroller.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	scController := &StorageClassReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
	}
	err = scController.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = mgr.Start(testCtx)
		if err != nil {
			mgr.GetLogger().Error(err, "failed to start manager")
		}
	}()

	By("setup test utilities")
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	testCancel()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
