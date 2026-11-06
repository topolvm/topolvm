package k8s

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme    = runtime.NewScheme()
	k8sClient client.Client
	testEnv   *envtest.Environment
)

func TestK8s(t *testing.T) {
	RegisterFailHandler(Fail)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(time.Minute)

	RunSpecs(t, "k8s Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:           []string{filepath.Join("..", "..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing:       true,
		DownloadBinaryAssets:        true,
		DownloadBinaryAssetsVersion: "v" + os.Getenv("ENVTEST_KUBERNETES_VERSION"),
		BinaryAssetsDirectory:       os.Getenv("ENVTEST_ASSETS_DIR"),
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(topolvmv1.AddToScheme(scheme)).To(Succeed())
	Expect(topolvmlegacyv1.AddToScheme(scheme)).To(Succeed())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())
})

var _ = Describe("CRD", func() {
	It("should accept a field selector on status.volumeID", func(ctx context.Context) {
		Expect(k8sClient.List(ctx, &topolvmv1.LogicalVolumeList{},
			client.MatchingFields{indexFieldVolumeID: "dummy"})).To(Succeed())
		Expect(k8sClient.List(ctx, &topolvmlegacyv1.LogicalVolumeList{},
			client.MatchingFields{indexFieldVolumeID: "dummy"})).To(Succeed())
	})
})
