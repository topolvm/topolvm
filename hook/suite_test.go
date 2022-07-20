package hook

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
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
var testCtx, testCancel = context.WithCancel(context.Background())

const (
	topolvmProvisionerStorageClassName          = "topolvm-provisioner"
	topolvmProvisioner2StorageClassName         = "topolvm-provisioner2"
	topolvmProvisioner3StorageClassName         = "topolvm-provisioner3"
	topolvmProvisionerImmediateStorageClassName = "topolvm-provisioner-immediate"
	hostLocalStorageClassName                   = "host-local"
	missingStorageClassName                     = "missing-storageclass"
)

var (
	podMutatingWebhookPath = "/pod/mutate"
	pvcMutatingWebhookPath = "/pvc/mutate"
)

func strPtr(s string) *string { return &s }

func modePtr(m storagev1.VolumeBindingMode) *storagev1.VolumeBindingMode { return &m }

func setupCommonResources() {
	// StrageClass
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolvmProvisionerStorageClassName,
		},
		Provisioner:       "topolvm.io",
		VolumeBindingMode: modePtr(storagev1.VolumeBindingWaitForFirstConsumer),
		Parameters: map[string]string{
			topolvm.GetDeviceClassKey(): "ssd",
		},
	}
	err := k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())

	sc = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolvmProvisioner2StorageClassName,
		},
		Provisioner:       "topolvm.io",
		VolumeBindingMode: modePtr(storagev1.VolumeBindingWaitForFirstConsumer),
		Parameters: map[string]string{
			topolvm.GetDeviceClassKey(): "hdd1",
		},
	}
	err = k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())

	sc = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolvmProvisioner3StorageClassName,
		},
		Provisioner:       "topolvm.io",
		VolumeBindingMode: modePtr(storagev1.VolumeBindingWaitForFirstConsumer),
		Parameters: map[string]string{
			topolvm.GetDeviceClassKey(): "hdd2",
		},
	}
	err = k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())

	sc = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolvmProvisionerImmediateStorageClassName,
		},
		Provisioner:       "topolvm.io",
		VolumeBindingMode: modePtr(storagev1.VolumeBindingImmediate),
		Parameters: map[string]string{
			topolvm.GetDeviceClassKey(): "ssd",
		},
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
	failPolicy := admissionv1.Fail
	sideEffects := admissionv1.SideEffectClassNone
	webhookInstallOptions := envtest.WebhookInstallOptions{
		MutatingWebhooks: []*admissionv1.MutatingWebhookConfiguration{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "topolvm-hook",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MutatingWebhookConfiguration",
					APIVersion: "admissionregistration.k8s.io/v1",
				},
				Webhooks: []admissionv1.MutatingWebhook{
					{
						Name:                    "pod-hook.topolvm.io",
						AdmissionReviewVersions: []string{"v1", "v1beta1"},
						FailurePolicy:           &failPolicy,
						ClientConfig: admissionv1.WebhookClientConfig{
							Service: &admissionv1.ServiceReference{
								Path: &podMutatingWebhookPath,
							},
						},
						Rules: []admissionv1.RuleWithOperations{
							{
								Operations: []admissionv1.OperationType{
									admissionv1.Create,
								},
								Rule: admissionv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
							},
						},
						SideEffects: &sideEffects,
					},
					{
						Name:                    "pvc-hook.topolvm.io",
						AdmissionReviewVersions: []string{"v1", "v1beta1"},
						FailurePolicy:           &failPolicy,
						ClientConfig: admissionv1.WebhookClientConfig{
							Service: &admissionv1.ServiceReference{
								Path: &pvcMutatingWebhookPath,
							},
						},
						Rules: []admissionv1.RuleWithOperations{
							{
								Operations: []admissionv1.OperationType{
									admissionv1.Create,
								},
								Rule: admissionv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"persistentvolumeclaims"},
								},
							},
						},
						SideEffects: &sideEffects,
					},
				},
			},
		},
	}

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		WebhookInstallOptions: webhookInstallOptions,
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
	go run(testCtx, cfg, scheme, &testEnv.WebhookInstallOptions)
	d := &net.Dialer{Timeout: time.Second}
	Eventually(func() error {
		serverURL := fmt.Sprintf("%s:%d", testEnv.WebhookInstallOptions.LocalServingHost, testEnv.WebhookInstallOptions.LocalServingPort)
		conn, err := tls.DialWithDialer(d, "tcp", serverURL, &tls.Config{
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
	testCancel()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
