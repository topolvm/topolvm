package hook

import (
	"context"
	"crypto/tls"
	"io/ioutil"
	"net"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var testCtx = context.Background()
var stopCh = make(chan struct{})

// copied from https://github.com/kubernetes-sigs/controller-runtime/blob/v0.5.0/pkg/internal/testing/integration/internal/apiserver.go
var apiServerDefaultArgs = []string{
	// Allow tests to run offline, by preventing API server from attempting to
	// use default route to determine its --advertise-address
	"--advertise-address=127.0.0.1",
	"--etcd-servers={{ if .EtcdURL }}{{ .EtcdURL.String }}{{ end }}",
	"--cert-dir={{ .CertDir }}",
	"--insecure-port={{ if .URL }}{{ .URL.Port }}{{ end }}",
	"--insecure-bind-address={{ if .URL }}{{ .URL.Hostname }}{{ end }}",
	"--secure-port={{ if .SecurePort }}{{ .SecurePort }}{{ end }}",
	"--admission-control=AlwaysAdmit",
	"--service-cluster-ip-range=10.0.0.0/24",
	"--allow-privileged=true",
}

const (
	topolvmProvisionerStorageClassName          = "topolvm-provisioner"
	topolvmProvisionerImmediateStorageClassName = "topolvm-provisioner-immediate"
	hostLocalStorageClassName                   = "host-local"
)

func strPtr(s string) *string { return &s }

func modePtr(m storagev1.VolumeBindingMode) *storagev1.VolumeBindingMode { return &m }

func setupCommonResources() {
	caBundle, err := ioutil.ReadFile("testdata/ca.crt")
	Expect(err).ShouldNot(HaveOccurred())
	wh := &admissionregistrationv1beta1.MutatingWebhookConfiguration{}
	wh.Name = "topolvm-hook"
	_, err = ctrl.CreateOrUpdate(testCtx, k8sClient, wh, func() error {
		failPolicy := admissionregistrationv1beta1.Fail
		wh.Webhooks = []admissionregistrationv1beta1.MutatingWebhook{
			{
				Name:          "pod-hook.topolvm.cybozu.com",
				FailurePolicy: &failPolicy,
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					CABundle: caBundle,
					URL:      strPtr("https://127.0.0.1:8443/pod/mutate"),
				},
				Rules: []admissionregistrationv1beta1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1beta1.OperationType{
							admissionregistrationv1beta1.Create,
						},
						Rule: admissionregistrationv1beta1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				},
			},
			{
				Name:          "pvc-hook.topolvm.cybozu.com",
				FailurePolicy: &failPolicy,
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					CABundle: caBundle,
					URL:      strPtr("https://127.0.0.1:8443/pvc/mutate"),
				},
				Rules: []admissionregistrationv1beta1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1beta1.OperationType{
							admissionregistrationv1beta1.Create,
						},
						Rule: admissionregistrationv1beta1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"persistentvolumeclaims"},
						},
					},
				},
			},
		}
		return nil
	})
	Expect(err).ShouldNot(HaveOccurred())

	// StrageClass
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolvmProvisionerStorageClassName,
		},
		Provisioner:       "topolvm.cybozu.com",
		VolumeBindingMode: modePtr(storagev1.VolumeBindingWaitForFirstConsumer),
	}
	err = k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())

	sc = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolvmProvisionerImmediateStorageClassName,
		},
		Provisioner:       "topolvm.cybozu.com",
		VolumeBindingMode: modePtr(storagev1.VolumeBindingImmediate),
	}
	err = k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())

	sc = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: hostLocalStorageClassName,
		},
		Provisioner: "kubernetes.io/no-provisioner",
	}
	err = k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())
}

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
	apiServerFlags := append(apiServerDefaultArgs,
		"--admission-control=MutatingAdmissionWebhook",
		"--feature-gates=CSIInlineVolume=true")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:  []string{filepath.Join("..", "config", "crd", "bases")},
		KubeAPIServerFlags: apiServerFlags,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	scheme := runtime.NewScheme()
	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).ToNot(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	By("running webhook server")
	go run(stopCh, cfg, scheme, "127.0.0.1", 8443)
	d := &net.Dialer{Timeout: time.Second}
	Eventually(func() error {
		conn, err := tls.DialWithDialer(d, "tcp", "127.0.0.1:8443", &tls.Config{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}).Should(Succeed())

	By("setting up resources")
	setupCommonResources()
	setupMutatePodResources()
	setupMutatePVCResources()
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	close(stopCh)
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
