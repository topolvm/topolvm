package v2

import (
	"context"
	"os"
	"testing"
	"time"

	snapapi "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	legacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	v1 "github.com/topolvm/topolvm/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// skipSpecs holds regexp strings that are used to skip tests.
// It is intended to be setup by init function on each test file if necessary.
var skipSpecs []string

var nonControlPlaneNodeCount int

var e2eclient crclient.Client

var podrunner *PodRunner

func TestMtest(t *testing.T) {
	if os.Getenv("E2ETEST") == "" {
		t.Skip("Run under test/e2e/")
	}

	RegisterFailHandler(Fail)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(time.Minute)

	suiteConfig, _ := GinkgoConfiguration()
	suiteConfig.SkipStrings = append(suiteConfig.SkipStrings, skipSpecs...)

	RunSpecs(t, "Test on sanity", suiteConfig)
}

var _ = SynchronizedBeforeSuite(func(ctx SpecContext) []byte {

	scheme := runtime.NewScheme()

	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := getKubeconfig(kubeconfig)
	Expect(err).ToNot(HaveOccurred(), "kubeconfig should be available for E2E tests")

	utilruntime.Must(k8sscheme.AddToScheme(scheme))
	utilruntime.Must(v1.AddToScheme(scheme))
	utilruntime.Must(legacyv1.AddToScheme(scheme))
	utilruntime.Must(snapapi.AddToScheme(scheme))

	podrunner, err = NewPodRunner(config, scheme)
	Expect(err).ToNot(HaveOccurred(), "podrunner should be available for E2E tests")

	e2eclient, err = crclient.New(config, crclient.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred(), "client should be available for E2E tests")

	By("Getting node count")
	sel, err := labels.Parse("!node-role.kubernetes.io/control-plane")
	Expect(err).ToNot(HaveOccurred())
	var nodes corev1.NodeList
	Expect(e2eclient.List(ctx, &nodes, crclient.MatchingLabelsSelector{Selector: sel})).To(Succeed())
	nonControlPlaneNodeCount = len(nodes.Items)

	By("Waiting for kindnet to get ready if necessary")
	// Because kindnet will crash. we need to confirm its readiness twice.
	Eventually(waitKindnet).Should(Succeed())
	time.Sleep(5 * time.Second)
	Eventually(waitKindnet).Should(Succeed())

	SetDefaultEventuallyTimeout(5 * time.Minute)

	By("Waiting for mutating webhook to get ready, by testing a pause Pod")
	Eventually(createPausePod).Should(Succeed())
	Eventually(deletePausePod).Should(Succeed())

	return nil
}, func(ctx SpecContext, data []byte) {

})

var _ = Describe("TopoLVM", func() {
	Context("e2e", testE2E)
})

func getKubeconfig(kubeconfig string) (*rest.Config, error) {
	var config *rest.Config
	var err error
	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}
	return config, err
}

func waitKindnet(g Gomega, ctx context.Context) {
	var nodes corev1.NodeList
	g.Expect(e2eclient.List(ctx, &nodes)).To(Succeed())
	var ds appsv1.DaemonSet
	err := e2eclient.Get(ctx, crclient.ObjectKey{
		Name:      "kindnet",
		Namespace: "kube-system",
	}, &ds)
	if crclient.IgnoreNotFound(err) != nil {
		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(ds.Status.NumberReady).To(BeEquivalentTo(len(nodes.Items)))
	}
}

var pausePod = corev1.Pod{
	TypeMeta: metav1.TypeMeta{},
	ObjectMeta: metav1.ObjectMeta{
		Name: "pause",
		Labels: map[string]string{
			"app.kubernetes.io/name": "pause",
		},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:  "pause",
			Image: "registry.k8s.io/pause",
		}},
	},
}

func createPausePod(g Gomega, ctx context.Context) {
	g.Expect(e2eclient.Create(ctx, pausePod.DeepCopy())).To(Succeed())
}

func deletePausePod(g Gomega, ctx context.Context) {
	g.Expect(e2eclient.Delete(ctx, pausePod.DeepCopy())).To(Succeed())
}
