package client

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	testingutil "github.com/topolvm/topolvm/util/testing"
	"google.golang.org/grpc/codes"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var scheme = runtime.NewScheme()

var k8sDelegatedClient client.Client
var k8sAPIReader client.Reader
var k8sCache cache.Cache

var testEnv *envtest.Environment
var testCtx, testCancel = context.WithCancel(context.Background())

func TestAPIs(t *testing.T) {
	testingutil.DoEnvCheck(t)
	RegisterFailHandler(Fail)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(time.Minute)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = topolvmv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = topolvmlegacyv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	c, err := cluster.New(cfg, func(clusterOptions *cluster.Options) {
		clusterOptions.Scheme = scheme
	})
	Expect(err).NotTo(HaveOccurred())
	k8sDelegatedClient = c.GetClient()
	k8sCache = c.GetCache()
	k8sAPIReader = c.GetAPIReader()

	go func() {
		err := k8sCache.Start(testCtx)
		Expect(err).NotTo(HaveOccurred())
	}()
	Expect(k8sCache.WaitForCacheSync(testCtx)).ToNot(BeFalse())

	scheme.Converter().WithConversions(conversion.NewConversionFuncs())
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	testCancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func currentLV(i int) *topolvmv1.LogicalVolume {
	return &topolvmv1.LogicalVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("current-%d", i),
		},
		Spec: topolvmv1.LogicalVolumeSpec{
			Name:        fmt.Sprintf("current-%d", i),
			NodeName:    fmt.Sprintf("node-%d", i),
			DeviceClass: topolvm.DefaultDeviceClassName,
			Size:        *resource.NewQuantity(1<<30, resource.BinarySI),
			Source:      fmt.Sprintf("source-%d", i),
			AccessType:  "rw",
		},
		Status: topolvmv1.LogicalVolumeStatus{
			VolumeID:    fmt.Sprintf("volume-%d", i),
			Code:        codes.Unknown,
			Message:     codes.Unknown.String(),
			CurrentSize: resource.NewQuantity(1<<30, resource.BinarySI),
		},
	}
}

func legacyLV(i int) *topolvmlegacyv1.LogicalVolume {
	return &topolvmlegacyv1.LogicalVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("legacy-%d", i),
		},
		Spec: topolvmlegacyv1.LogicalVolumeSpec{
			Name:        fmt.Sprintf("legacy-%d", i),
			NodeName:    fmt.Sprintf("node-%d", i),
			DeviceClass: topolvm.DefaultDeviceClassName,
			Size:        *resource.NewQuantity(1<<30, resource.BinarySI),
			Source:      fmt.Sprintf("source-%d", i),
			AccessType:  "rw",
		},
		Status: topolvmlegacyv1.LogicalVolumeStatus{
			VolumeID:    fmt.Sprintf("volume-%d", i),
			Code:        codes.Unknown,
			Message:     codes.Unknown.String(),
			CurrentSize: resource.NewQuantity(1<<30, resource.BinarySI),
		},
	}
}

func setCurrentLVStatus(lv *topolvmv1.LogicalVolume, i int) {
	lv.Status = topolvmv1.LogicalVolumeStatus{
		VolumeID:    fmt.Sprintf("volume-%d", i),
		Code:        codes.Unknown,
		Message:     codes.Unknown.String(),
		CurrentSize: resource.NewQuantity(1<<30, resource.BinarySI),
	}
}

func setLegacyLVStatus(lv *topolvmlegacyv1.LogicalVolume, i int) {
	lv.Status = topolvmlegacyv1.LogicalVolumeStatus{
		VolumeID:    fmt.Sprintf("volume-%d", i),
		Code:        codes.Unknown,
		Message:     codes.Unknown.String(),
		CurrentSize: resource.NewQuantity(1<<30, resource.BinarySI),
	}
}

func convertToCurrent(lv *topolvmlegacyv1.LogicalVolume) *topolvmv1.LogicalVolume {
	u := &unstructured.Unstructured{}
	err := k8sDelegatedClient.Scheme().Convert(lv, u, nil)
	Expect(err).ShouldNot(HaveOccurred())
	u.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(logicalVolume))
	current := new(topolvmv1.LogicalVolume)
	err = k8sDelegatedClient.Scheme().Convert(u, current, nil)
	Expect(err).ShouldNot(HaveOccurred())
	return current
}
