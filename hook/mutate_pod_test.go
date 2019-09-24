package hook

import (
	"context"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var testCtx = context.Background()

func strPtr(s string) *string { return &s }

func pvcSource(name string) *corev1.PersistentVolumeClaimVolumeSource {
	return &corev1.PersistentVolumeClaimVolumeSource{
		ClaimName: name,
	}
}

func setupResources() {
	caBundle, err := ioutil.ReadFile("certs/ca.crt")
	Expect(err).ShouldNot(HaveOccurred())
	wh := &admissionregistrationv1beta1.MutatingWebhookConfiguration{}
	wh.Name = "topolvm-hook"
	_, err = ctrl.CreateOrUpdate(testCtx, k8sClient, wh, func() error {
		failPolicy := admissionregistrationv1beta1.Fail
		urlStr := "https://127.0.0.1:8443/mutate-pod"
		wh.Webhooks = []admissionregistrationv1beta1.MutatingWebhook{
			{
				Name:          "hook.topolvm.cybozu.com",
				FailurePolicy: &failPolicy,
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					CABundle: caBundle,
					URL:      &urlStr,
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
		}
		return nil
	})
	Expect(err).ShouldNot(HaveOccurred())

	sc := &storagev1.StorageClass{}
	sc.Name = "topolvm"
	sc.Provisioner = "topolvm.cybozu.com"
	mode := storagev1.VolumeBindingWaitForFirstConsumer
	sc.VolumeBindingMode = &mode
	err = k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())

	sc = &storagev1.StorageClass{}
	sc.Name = "host-local"
	sc.Provisioner = "kubernetes.io/no-provisioner"
	err = k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())

	// Namespace and namespace resources
	ns := &corev1.Namespace{}
	ns.Name = "test"
	err = k8sClient.Create(testCtx, ns)
	Expect(err).ShouldNot(HaveOccurred())

	localPVC := &corev1.PersistentVolumeClaim{}
	localPVC.Namespace = "test"
	localPVC.Name = "local-pvc"
	localPVC.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	localPVC.Spec.StorageClassName = strPtr("host-local")
	localPVC.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(10<<30, resource.DecimalSI),
	}
	err = k8sClient.Create(testCtx, localPVC)
	Expect(err).ShouldNot(HaveOccurred())

	boundPVC := &corev1.PersistentVolumeClaim{}
	boundPVC.Namespace = "test"
	boundPVC.Name = "bound-pvc"
	boundPVC.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	boundPVC.Spec.StorageClassName = strPtr("topolvm")
	boundPVC.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(100<<30, resource.DecimalSI),
	}
	err = k8sClient.Create(testCtx, boundPVC)
	Expect(err).ShouldNot(HaveOccurred())

	// set PVC status
	boundPVC.Status.Phase = corev1.ClaimBound
	err = k8sClient.Status().Update(testCtx, boundPVC)
	Expect(err).ShouldNot(HaveOccurred())

	pvc1 := &corev1.PersistentVolumeClaim{}
	pvc1.Namespace = "test"
	pvc1.Name = "pvc1"
	pvc1.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	pvc1.Spec.StorageClassName = strPtr("topolvm")
	pvc1.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(100<<20, resource.DecimalSI),
	}
	err = k8sClient.Create(testCtx, pvc1)
	Expect(err).ShouldNot(HaveOccurred())

	pvc2 := &corev1.PersistentVolumeClaim{}
	pvc2.Namespace = "test"
	pvc2.Name = "pvc2"
	pvc2.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	pvc2.Spec.StorageClassName = strPtr("topolvm")
	pvc2.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(2<<30-1, resource.DecimalSI),
	}
	err = k8sClient.Create(testCtx, pvc2)
	Expect(err).ShouldNot(HaveOccurred())

	defaultPVC := &corev1.PersistentVolumeClaim{}
	defaultPVC.Namespace = "test"
	defaultPVC.Name = "default-pvc"
	defaultPVC.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	defaultPVC.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(3<<30, resource.DecimalSI),
	}
	err = k8sClient.Create(testCtx, defaultPVC)
	Expect(err).ShouldNot(HaveOccurred())
}

func testPod() *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Namespace = "test"
	pod.Name = "test"
	pod.Spec.Containers = []corev1.Container{
		{
			Name:  "container1",
			Image: "ubuntu",
		},
		{
			Name:  "container2",
			Image: "ubuntu",
		},
	}
	return pod
}

func getPod() *corev1.Pod {
	pod := &corev1.Pod{}
	name := types.NamespacedName{
		Namespace: "test",
		Name:      "test",
	}
	err := k8sClient.Get(testCtx, name, pod)
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())
	return pod
}

var _ = Describe("pod mutation webhook", func() {
	AfterEach(func() {
		pod := &corev1.Pod{}
		pod.Name = "test"
		pod.Namespace = "test"
		err := k8sClient.Delete(testCtx, pod, client.GracePeriodSeconds(0))
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("should not mutate pod w/o PVC", func() {
		pod := testPod()
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		Expect(pod.Spec.Containers[0].Resources.Requests).To(BeEmpty())
		Expect(pod.Spec.Containers[0].Resources.Limits).To(BeEmpty())
	})

	It("should mutate pod w/ TopoLVM PVC", func() {
		pod := testPod()
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "vol1",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: pvcSource("pvc1"),
				},
			},
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		request := pod.Spec.Containers[0].Resources.Requests["topolvm.cybozu.com/capacity"]
		limit := pod.Spec.Containers[0].Resources.Limits["topolvm.cybozu.com/capacity"]
		Expect(request.Value()).Should(BeNumerically("==", 1<<30))
		Expect(limit.Value()).Should(BeNumerically("==", 1<<30))
	})

	It("should keep existing resources", func() {
		pod := testPod()
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "vol1",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: pvcSource("pvc1"),
				},
			},
		}
		pod.Spec.Containers[0].Resources.Requests = corev1.ResourceList{
			"memory": *resource.NewQuantity(100, resource.DecimalSI),
		}
		pod.Spec.Containers[0].Resources.Limits = corev1.ResourceList{
			"memory": *resource.NewQuantity(100, resource.DecimalSI),
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		request := pod.Spec.Containers[0].Resources.Requests["topolvm.cybozu.com/capacity"]
		limit := pod.Spec.Containers[0].Resources.Limits["topolvm.cybozu.com/capacity"]
		Expect(request.Value()).Should(BeNumerically("==", 1<<30))
		Expect(limit.Value()).Should(BeNumerically("==", 1<<30))

		mem := pod.Spec.Containers[0].Resources.Requests["memory"]
		Expect(mem.Value()).Should(BeNumerically("==", 100))
		mem = pod.Spec.Containers[0].Resources.Limits["memory"]
		Expect(mem.Value()).Should(BeNumerically("==", 100))
	})

	It("should not mutate pod w/ bound TopoLVM PVC", func() {
		pod := testPod()
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "vol1",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: pvcSource("pvc1"),
				},
			},
			{
				Name: "vol2",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: pvcSource("bound-pvc"),
				},
			},
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		Expect(pod.Spec.Containers[0].Resources.Requests).To(BeEmpty())
		Expect(pod.Spec.Containers[0].Resources.Limits).To(BeEmpty())
	})

	It("should calculate requested capacity correctly", func() {
		pod := testPod()
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "vol1",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: pvcSource("local-pvc"),
				},
			},
			{
				Name: "vol2",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: pvcSource("pvc1"),
				},
			},
			{
				Name: "vol3",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: pvcSource("pvc2"),
				},
			},
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		request := pod.Spec.Containers[0].Resources.Requests["topolvm.cybozu.com/capacity"]
		Expect(request.Value()).Should(BeNumerically("==", 3<<30))
	})

	It("should handle PVC w/o storage class", func() {
		pod := testPod()
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "vol1",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: pvcSource("default-pvc"),
				},
			},
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		Expect(pod.Spec.Containers[0].Resources.Requests).To(BeEmpty())
		Expect(pod.Spec.Containers[0].Resources.Limits).To(BeEmpty())
	})
})
