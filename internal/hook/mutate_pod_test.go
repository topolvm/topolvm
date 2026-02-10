package hook

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	mutatePodNamespace = "test-mutate-pod"
	defaultPodName     = "test-pod"
	mebibyte           = 1048576
	deviceClass1       = "dc1"
	deviceClass2       = "dc2"
	deviceClass3       = "dc3"
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
	localPVC.Spec.StorageClassName = ptr.To(hostLocalStorageClassName)
	localPVC.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(10<<30, resource.BinarySI),
	}
	err = k8sClient.Create(testCtx, localPVC)
	Expect(err).ShouldNot(HaveOccurred())

	boundPVC := &corev1.PersistentVolumeClaim{}
	boundPVC.Namespace = mutatePodNamespace
	boundPVC.Name = "bound-pvc"
	boundPVC.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	boundPVC.Spec.StorageClassName = ptr.To(topolvmProvisionerStorageClassName)
	boundPVC.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(100<<30, resource.BinarySI),
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
	pvc1.Spec.StorageClassName = ptr.To(topolvmProvisionerStorageClassName)
	pvc1.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(100<<30, resource.BinarySI),
	}
	err = k8sClient.Create(testCtx, pvc1)
	Expect(err).ShouldNot(HaveOccurred())

	pvc2 := &corev1.PersistentVolumeClaim{}
	pvc2.Namespace = mutatePodNamespace
	pvc2.Name = "pvc2"
	pvc2.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	pvc2.Spec.StorageClassName = ptr.To(topolvmProvisionerStorageClassName)
	pvc2.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(2<<30-1, resource.BinarySI),
	}
	err = k8sClient.Create(testCtx, pvc2)
	Expect(err).ShouldNot(HaveOccurred())

	pvc3 := &corev1.PersistentVolumeClaim{}
	pvc3.Namespace = mutatePodNamespace
	pvc3.Name = "pvc3"
	pvc3.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	pvc3.Spec.StorageClassName = ptr.To(topolvmProvisioner2StorageClassName)
	pvc3.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(3<<30, resource.BinarySI),
	}
	err = k8sClient.Create(testCtx, pvc3)
	Expect(err).ShouldNot(HaveOccurred())

	pvc4 := &corev1.PersistentVolumeClaim{}
	pvc4.Namespace = mutatePodNamespace
	pvc4.Name = "pvc4"
	pvc4.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	pvc4.Spec.StorageClassName = ptr.To(topolvmProvisioner3StorageClassName)
	pvc4.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(4<<30, resource.BinarySI),
	}
	err = k8sClient.Create(testCtx, pvc4)
	Expect(err).ShouldNot(HaveOccurred())

	pvc5 := &corev1.PersistentVolumeClaim{}
	pvc5.Namespace = mutatePodNamespace
	pvc5.Name = "pvc5"
	pvc5.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	pvc5.Spec.StorageClassName = ptr.To(topolvmProvisionerStorageClassName)
	pvc5.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(500*mebibyte, resource.BinarySI),
	}
	err = k8sClient.Create(testCtx, pvc5)
	Expect(err).ShouldNot(HaveOccurred())

	defaultPVC := &corev1.PersistentVolumeClaim{}
	defaultPVC.Namespace = mutatePodNamespace
	defaultPVC.Name = "default-pvc"
	defaultPVC.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	defaultPVC.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(3<<30, resource.BinarySI),
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
		Expect(pod.Annotations).NotTo(HaveKey(topolvm.GetCapacityKeyPrefix()))
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
		request := pod.Spec.Containers[0].Resources.Requests[topolvm.GetCapacityResource()]
		limit := pod.Spec.Containers[0].Resources.Limits[topolvm.GetCapacityResource()]
		capacity := pod.Annotations[topolvm.GetCapacityKeyPrefix()+deviceClass1]
		Expect(request.Value()).Should(Equal(int64(1)))
		Expect(limit.Value()).Should(Equal(int64(1)))
		Expect(capacity).Should(Equal(strconv.Itoa(100 << 30)))
	})

	It("should mutate pod w/ TopoLVM PVC on multiple volume groups", func() {
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
					PersistentVolumeClaim: pvcSource("pvc3"),
				},
			},
			{
				Name: "vol3",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: pvcSource("pvc4"),
				},
			},
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		request := pod.Spec.Containers[0].Resources.Requests[topolvm.GetCapacityResource()]
		limit := pod.Spec.Containers[0].Resources.Limits[topolvm.GetCapacityResource()]
		capacity := pod.Annotations[topolvm.GetCapacityKeyPrefix()+deviceClass1]
		Expect(request.Value()).Should(Equal(int64(1)))
		Expect(limit.Value()).Should(Equal(int64(1)))
		Expect(capacity).Should(Equal(strconv.Itoa(100 << 30)))

		request = pod.Spec.Containers[0].Resources.Requests[topolvm.GetCapacityResource()]
		limit = pod.Spec.Containers[0].Resources.Limits[topolvm.GetCapacityResource()]
		capacity = pod.Annotations[topolvm.GetCapacityKeyPrefix()+deviceClass2]
		Expect(request.Value()).Should(Equal(int64(1)))
		Expect(limit.Value()).Should(Equal(int64(1)))
		Expect(capacity).Should(Equal(strconv.Itoa(3 << 30)))

		request = pod.Spec.Containers[0].Resources.Requests[topolvm.GetCapacityResource()]
		capacity = pod.Annotations[topolvm.GetCapacityKeyPrefix()+deviceClass3]
		limit = pod.Spec.Containers[0].Resources.Limits[topolvm.GetCapacityResource()]
		Expect(request.Value()).Should(Equal(int64(1)))
		Expect(limit.Value()).Should(Equal(int64(1)))
		Expect(capacity).Should(Equal(strconv.Itoa(4 << 30)))
	})

	It("should mutate pod with generic ephemeral volume.", func() {
		pod := testPod()
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "my-volume",
				VolumeSource: corev1.VolumeSource{
					Ephemeral: &corev1.EphemeralVolumeSource{
						VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								StorageClassName: ptr.To(topolvmProvisionerStorageClassName),
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": *resource.NewQuantity(100<<30, resource.BinarySI),
									},
								},
							},
						},
					},
				},
			},
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		request := pod.Spec.Containers[0].Resources.Requests[topolvm.GetCapacityResource()]
		limit := pod.Spec.Containers[0].Resources.Limits[topolvm.GetCapacityResource()]
		capacity := pod.Annotations[topolvm.GetCapacityKeyPrefix()+deviceClass1]
		Expect(request.Value()).Should(Equal(int64(1)))
		Expect(limit.Value()).Should(Equal(int64(1)))
		Expect(capacity).Should(Equal(strconv.Itoa(100 << 30)))
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
			"memory": *resource.NewQuantity(100, resource.BinarySI),
		}
		pod.Spec.Containers[0].Resources.Limits = corev1.ResourceList{
			"memory": *resource.NewQuantity(100, resource.BinarySI),
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		request := pod.Spec.Containers[0].Resources.Requests[topolvm.GetCapacityResource()]
		limit := pod.Spec.Containers[0].Resources.Limits[topolvm.GetCapacityResource()]
		capacity := pod.Annotations[topolvm.GetCapacityKeyPrefix()+deviceClass1]
		Expect(request.Value()).Should(Equal(int64(1)))
		Expect(limit.Value()).Should(Equal(int64(1)))
		Expect(capacity).Should(Equal(strconv.Itoa(100 << 30)))

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
		Expect(pod.Annotations).NotTo(HaveKey(topolvm.GetCapacityKeyPrefix()))
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
					PersistentVolumeClaim: pvcSource("pvc1"), // 100<<30 capacity
				},
			},
			{
				Name: "vol3",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: pvcSource("pvc2"), // 2<<30-1 capacity
				},
			},
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		request := pod.Spec.Containers[0].Resources.Requests[topolvm.GetCapacityResource()]
		capacity := pod.Annotations[topolvm.GetCapacityKeyPrefix()+deviceClass1]
		Expect(request.Value()).Should(Equal(int64(1)))
		Expect(capacity).Should(Equal(strconv.Itoa((100 << 30) + (2<<30 - 1))))
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
		Expect(pod.Annotations).NotTo(HaveKey(topolvm.GetCapacityKeyPrefix()))
	})

	It("should mutate pod w/ TopoLVM PVC for Storage Requests <1Gi", func() {
		pod := testPod()
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "vol1",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: pvcSource("pvc5"),
				},
			},
		}
		err := k8sClient.Create(testCtx, pod)
		Expect(err).ShouldNot(HaveOccurred())

		pod = getPod()
		request := pod.Spec.Containers[0].Resources.Requests[topolvm.GetCapacityResource()]
		limit := pod.Spec.Containers[0].Resources.Limits[topolvm.GetCapacityResource()]
		capacity := pod.Annotations[topolvm.GetCapacityKeyPrefix()+deviceClass1]
		Expect(request.Value()).Should(Equal(int64(1)))
		Expect(limit.Value()).Should(Equal(int64(1)))
		Expect(capacity).Should(Equal(strconv.Itoa(500 * mebibyte)))
	})
})
