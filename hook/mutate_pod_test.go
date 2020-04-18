package hook

import (
	"strconv"

	"github.com/cybozu-go/topolvm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	mutatePodNamespace = "test-mutate-pod"
	defaultPodName     = "test-pod"
)

func pvcSource(name string) *corev1.PersistentVolumeClaimVolumeSource {
	return &corev1.PersistentVolumeClaimVolumeSource{
		ClaimName: name,
	}
}

func setupMutatePodResources() {
	// Namespace and namespace resources
	ns := &corev1.Namespace{}
	ns.Name = mutatePodNamespace
	err := k8sClient.Create(testCtx, ns)
	Expect(err).ShouldNot(HaveOccurred())

	localPVC := &corev1.PersistentVolumeClaim{}
	localPVC.Namespace = mutatePodNamespace
	localPVC.Name = "local-pvc"
	localPVC.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	localPVC.Spec.StorageClassName = strPtr(hostLocalStorageClassName)
	localPVC.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(10<<30, resource.DecimalSI),
	}
	err = k8sClient.Create(testCtx, localPVC)
	Expect(err).ShouldNot(HaveOccurred())

	boundPVC := &corev1.PersistentVolumeClaim{}
	boundPVC.Namespace = mutatePodNamespace
	boundPVC.Name = "bound-pvc"
	boundPVC.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	boundPVC.Spec.StorageClassName = strPtr(topolvmProvisionerStorageClassName)
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
	pvc1.Namespace = mutatePodNamespace
	pvc1.Name = "pvc1"
	pvc1.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	pvc1.Spec.StorageClassName = strPtr(topolvmProvisionerStorageClassName)
	pvc1.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(100<<20, resource.DecimalSI),
	}
	err = k8sClient.Create(testCtx, pvc1)
	Expect(err).ShouldNot(HaveOccurred())

	pvc2 := &corev1.PersistentVolumeClaim{}
	pvc2.Namespace = mutatePodNamespace
	pvc2.Name = "pvc2"
	pvc2.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	pvc2.Spec.StorageClassName = strPtr(topolvmProvisionerStorageClassName)
	pvc2.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(2<<30-1, resource.DecimalSI),
	}
	err = k8sClient.Create(testCtx, pvc2)
	Expect(err).ShouldNot(HaveOccurred())

	defaultPVC := &corev1.PersistentVolumeClaim{}
	defaultPVC.Namespace = mutatePodNamespace
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
	pod.Namespace = mutatePodNamespace
	pod.Name = defaultPodName
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
		Namespace: mutatePodNamespace,
		Name:      defaultPodName,
	}
	err := k8sClient.Get(testCtx, name, pod)
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())
	return pod
}

var _ = Describe("pod mutation webhook", func() {
	AfterEach(func() {
		pod := &corev1.Pod{}
		pod.Namespace = mutatePodNamespace
		pod.Name = defaultPodName
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
		Expect(pod.Annotations).NotTo(HaveKey(topolvm.CapacityKey))
	})

	It("should create pod before its PVC", func() {
		pod := testPod()
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "vol1",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: pvcSource("non-existent"),
				},
			},
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())
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
		request := pod.Spec.Containers[0].Resources.Requests[topolvm.CapacityKey]
		limit := pod.Spec.Containers[0].Resources.Limits[topolvm.CapacityKey]
		capacity := pod.Annotations[topolvm.CapacityKey+"-myvg1"]
		Expect(request.Value()).Should(BeNumerically("==", 1))
		Expect(limit.Value()).Should(BeNumerically("==", 1))
		Expect(capacity).Should(Equal(strconv.Itoa(1 << 30)))
	})

	It("should mutate pod with TopoLVM inline ephemeral volume.", func() {
		pod := testPod()
		fsType := "xfs"
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "my-volume",
				VolumeSource: corev1.VolumeSource{
					CSI: &corev1.CSIVolumeSource{
						Driver:           "topolvm.cybozu.com",
						FSType:           &fsType,
						VolumeAttributes: map[string]string{topolvm.SizeVolConKey: "2"},
					},
				},
			},
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		request := pod.Spec.Containers[0].Resources.Requests[topolvm.CapacityKey]
		limit := pod.Spec.Containers[0].Resources.Limits[topolvm.CapacityKey]
		capacity := pod.Annotations[topolvm.CapacityKey+"-myvg1"]
		Expect(request.Value()).Should(BeNumerically("==", 1))
		Expect(limit.Value()).Should(BeNumerically("==", 1))
		Expect(capacity).Should(Equal(strconv.Itoa(2 << 30)))
	})

	It("should mutate pod with TopoLVM inline ephemeral volume when volume size is not explicitly specified.", func() {
		pod := testPod()
		fsType := "xfs"
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "my-volume",
				VolumeSource: corev1.VolumeSource{
					CSI: &corev1.CSIVolumeSource{
						Driver: "topolvm.cybozu.com",
						FSType: &fsType,
						// Intentionally do not define size.
					},
				},
			},
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		request := pod.Spec.Containers[0].Resources.Requests[topolvm.CapacityKey]
		limit := pod.Spec.Containers[0].Resources.Limits[topolvm.CapacityKey]
		capacity := pod.Annotations[topolvm.CapacityKey+"-myvg1"]
		Expect(request.Value()).Should(BeNumerically("==", 1))
		Expect(limit.Value()).Should(BeNumerically("==", 1))
		Expect(capacity).Should(Equal(strconv.Itoa(1 << 30)))
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
		request := pod.Spec.Containers[0].Resources.Requests[topolvm.CapacityKey]
		limit := pod.Spec.Containers[0].Resources.Limits[topolvm.CapacityKey]
		capacity := pod.Annotations[topolvm.CapacityKey+"-myvg1"]
		Expect(request.Value()).Should(BeNumerically("==", 1))
		Expect(limit.Value()).Should(BeNumerically("==", 1))
		Expect(capacity).Should(Equal(strconv.Itoa(1 << 30)))

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
		Expect(pod.Annotations).NotTo(HaveKey(topolvm.CapacityKey))
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
		request := pod.Spec.Containers[0].Resources.Requests[topolvm.CapacityKey]
		Expect(request.Value()).Should(BeNumerically("==", 1))
		capacity := pod.Annotations[topolvm.CapacityKey+"-myvg1"]
		Expect(capacity).Should(Equal(strconv.Itoa(3 << 30)))
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
		Expect(pod.Annotations).NotTo(HaveKey(topolvm.CapacityKey))
	})
})
