package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var reconciler *PersistentVolumeClaimReconciler

var _ = Describe("PersistentVolumeClaimController controller", func() {
	ctx := context.Background()
	var stopFunc func()
	errCh := make(chan error)

	BeforeEach(func() {
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme,
		})
		Expect(err).ToNot(HaveOccurred())

		reconciler = NewPersistentVolumeClaimReconciler(k8sClient, mgr.GetAPIReader())
		err = reconciler.SetupWithManager(mgr)
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithCancel(ctx)
		stopFunc = cancel
		go func() {
			errCh <- mgr.Start(ctx)
		}()
		time.Sleep(100 * time.Millisecond)
	})

	AfterEach(func() {
		stopFunc()
		Expect(<-errCh).NotTo(HaveOccurred())
	})

	It("should list only the pods that are related to the PVC by getPodsByPVC", func() {
		ctx := context.Background()
		ns1 := createNamespace()
		ns2 := createNamespace()
		pods := []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod0",
					Namespace: ns1,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container",
							Image: "registry.k8s.io/pause",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "vol0",
									MountPath: "/vol0",
								},
								{
									Name:      "vol1",
									MountPath: "/vol1",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "vol0",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/dummy",
								},
							},
						},
						{
							Name: "vol1",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc1",
								},
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: ns1,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container",
							Image: "registry.k8s.io/pause",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "vol0",
									MountPath: "/vol0",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "vol0",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/dummy",
								},
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod2",
					Namespace: ns1,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container",
							Image: "registry.k8s.io/pause",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "vol1",
									MountPath: "/vol1",
								},
								{
									Name:      "vol2",
									MountPath: "/vol2",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "vol1",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc1",
								},
							},
						},
						{
							Name: "vol2",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc2",
								},
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod4",
					Namespace: ns2,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container",
							Image: "registry.k8s.io/pause",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "vol1",
									MountPath: "/vol1",
								},
								{
									Name:      "vol2",
									MountPath: "/vol2",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "vol1",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc1",
								},
							},
						},
						{
							Name: "vol2",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc2",
								},
							},
						},
					},
				},
			},
		}

		for _, pod := range pods {
			err := k8sClient.Create(ctx, &pod)
			Expect(err).NotTo(HaveOccurred())
		}

		tests := []struct {
			pvcMeta metav1.ObjectMeta
			expect  []metav1.ObjectMeta
		}{
			{
				pvcMeta: metav1.ObjectMeta{
					Name:      "pvc1",
					Namespace: ns1,
				},
				expect: []metav1.ObjectMeta{
					{
						Name:      "pod0",
						Namespace: ns1,
					},
					{
						Name:      "pod2",
						Namespace: ns1,
					},
				},
			},
			{
				pvcMeta: metav1.ObjectMeta{
					Name:      "not-exist",
					Namespace: ns1,
				},
				expect: []metav1.ObjectMeta{},
			},
		}
		for _, test := range tests {

			pvc := corev1.PersistentVolumeClaim{
				ObjectMeta: test.pvcMeta,
				Spec:       corev1.PersistentVolumeClaimSpec{},
			}
			pods, err := reconciler.getPodsByPVC(ctx, &pvc)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(pods)).Should(Equal(len(test.expect)))
			for _, pod := range pods {
				contained := false
				for _, expectPod := range test.expect {
					if pod.Name == expectPod.Name && pod.Namespace == expectPod.Namespace {
						contained = true
						break
					}
				}
				Expect(contained).Should(BeTrue())
			}
		}
	})

	It("should remove deprecated finalizer", func() {
		ctx := context.Background()
		ns := createNamespace()
		testCases := []struct {
			title            string
			pvcMeta          metav1.ObjectMeta
			expectFinalizers []string
		}{
			{
				title: "empty finalizers",
				pvcMeta: metav1.ObjectMeta{
					Name:       "pvc1",
					Namespace:  ns,
					Finalizers: []string{},
				},
				expectFinalizers: []string{
					"kubernetes.io/pvc-protection",
				},
			},
			{
				title: "there is an only foreign finalizer",
				pvcMeta: metav1.ObjectMeta{
					Name:      "pvc2",
					Namespace: ns,
					Finalizers: []string{
						"dummy/dummy",
					},
				},
				expectFinalizers: []string{
					"dummy/dummy",
					"kubernetes.io/pvc-protection",
				},
			},
			{
				title: "combination of foreign and deprecated finalizers",
				pvcMeta: metav1.ObjectMeta{
					Name:      "pvc3",
					Namespace: ns,
					Finalizers: []string{
						"dummy/dummy",
						"topolvm.cybozu.com/pvc",
					},
				},
				expectFinalizers: []string{
					"dummy/dummy",
					"kubernetes.io/pvc-protection",
				},
			},
			{
				title: "there is an only new finalizer",
				pvcMeta: metav1.ObjectMeta{
					Name:      "pvc4",
					Namespace: ns,
					Finalizers: []string{
						"topolvm.io/pvc",
					},
				},
				expectFinalizers: []string{
					"topolvm.io/pvc",
					"kubernetes.io/pvc-protection",
				},
			},
			{
				title: "there are same old finalizers",
				pvcMeta: metav1.ObjectMeta{
					Name:      "pvc5",
					Namespace: ns,
					Finalizers: []string{
						"dummy/dummy",
						"topolvm.cybozu.com/pvc",
						"topolvm.cybozu.com/pvc",
					},
				},
				expectFinalizers: []string{
					"dummy/dummy",
					"kubernetes.io/pvc-protection",
				},
			},
			{
				title: "there are old finalizer and new finalizer",
				pvcMeta: metav1.ObjectMeta{
					Name:      "pvc6",
					Namespace: ns,
					Finalizers: []string{
						"dummy/dummy",
						"topolvm.cybozu.com/pvc",
						"topolvm.io/pvc",
					},
				},
				expectFinalizers: []string{
					"dummy/dummy",
					"topolvm.io/pvc",
					"kubernetes.io/pvc-protection",
				},
			},
		}

		for _, testPVC := range testCases {
			pvc := corev1.PersistentVolumeClaim{
				ObjectMeta: testPVC.pvcMeta,
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": resource.MustParse("1Gi"),
						},
					},
				},
			}
			err := k8sClient.Create(ctx, &pvc)
			Expect(err).NotTo(HaveOccurred())
		}

		for _, testPVC := range testCases {
			pvc := corev1.PersistentVolumeClaim{}

			Eventually(func(g Gomega) []string {

				err := k8sClient.Get(ctx, client.ObjectKey{Name: testPVC.pvcMeta.Name, Namespace: ns}, &pvc)
				g.Expect(err).NotTo(HaveOccurred())

				return pvc.Finalizers
			}).Should(Equal(testPVC.expectFinalizers), testPVC.title)
		}

	})

	It("should be deleted Pods which is using PVC that has finalizer", func() {
		ctx := context.Background()
		ns1 := createNamespace()
		ns2 := createNamespace()

		pods := []corev1.Pod{
			// this pod will be deleted.
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod0",
					Namespace: ns1,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container",
							Image: "registry.k8s.io/pause",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "vol0",
									MountPath: "/vol0",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "vol0",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc0",
								},
							},
						},
					},
				},
			},
			// this pod will not be deleted because this pod doesn't used TopoLVM's PVC
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: ns1,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container",
							Image: "registry.k8s.io/pause",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "vol1",
									MountPath: "/vol1",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "vol1",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc1",
								},
							},
						},
					},
				},
			},
			// this pod will not be delete because this pod exists in another namespace from the pod will be deleted
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod0",
					Namespace: ns2,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:         "container",
							Image:        "registry.k8s.io/pause",
							VolumeMounts: []corev1.VolumeMount{},
						},
					},
					Volumes: []corev1.Volume{},
				},
			},
		}

		for _, pod := range pods {
			err := k8sClient.Create(ctx, &pod)
			Expect(err).NotTo(HaveOccurred())
		}

		pvcMetas := []metav1.ObjectMeta{
			// this PVC has a finalizer of TopoLVM
			{
				Name:      "pvc0",
				Namespace: ns1,
				Finalizers: []string{
					"topolvm.io/pvc",
				},
			},
			// this PVC doesn't have a finalizer of TopoLVM
			{
				Name:       "pvc1",
				Namespace:  ns1,
				Finalizers: []string{},
			},
		}

		for _, pvcMeta := range pvcMetas {
			pvc := corev1.PersistentVolumeClaim{
				ObjectMeta: pvcMeta,
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": resource.MustParse("1Gi"),
						},
					},
				},
			}
			err := k8sClient.Create(ctx, &pvc)
			Expect(err).NotTo(HaveOccurred())
		}

		for _, pvcMeta := range pvcMetas {

			err := k8sClient.Delete(ctx, &corev1.PersistentVolumeClaim{
				ObjectMeta: pvcMeta,
			})
			Expect(err).NotTo(HaveOccurred())

			getPods := func(g Gomega) []string {
				pods := []string{}

				ns1pods := corev1.PodList{}
				err := k8sClient.List(ctx, &ns1pods, client.InNamespace(ns1))
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range ns1pods.Items {
					pods = append(pods, pod.Namespace+"/"+pod.Name)
				}

				ns2pods := corev1.PodList{}
				err = k8sClient.List(ctx, &ns2pods, client.InNamespace(ns2))
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range ns2pods.Items {
					pods = append(pods, pod.Namespace+"/"+pod.Name)
				}

				return pods
			}

			Eventually(getPods).Should(Equal([]string{
				ns1 + "/pod1",
				ns2 + "/pod0",
			}), "when delete "+pvcMeta.Namespace+"/"+pvcMeta.Name)

			Consistently(getPods, 10*time.Second, 1*time.Second).Should(Equal([]string{
				ns1 + "/pod1",
				ns2 + "/pod0",
			}), "when delete "+pvcMeta.Namespace+"/"+pvcMeta.Name)
		}
	})
})
