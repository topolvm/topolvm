package controllers

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/topolvm/topolvm"
)

const (
	testProvisonerIDPrefix string = "1657038133042-8081-"
)

func testPV() *corev1.PersistentVolume {
	mode := corev1.PersistentVolumeFilesystem
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pv",
			Finalizers: []string{
				"kubernetes.io/pv-protection",
			},
			Annotations: map[string]string{
				AnnDynamicallyProvisioned: topolvm.LegacyPluginName,
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Capacity: corev1.ResourceList{
				"storage": resource.MustParse("1Gi"),
			},
			ClaimRef: &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "PersistentVolumeClaim",
				Name:       "test-pv-pvc",
				Namespace:  "default",
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver: topolvm.LegacyPluginName,
					FSType: "xfs",
					VolumeAttributes: map[string]string{
						provisionerIDKey: fmt.Sprintf("%s%s", testProvisonerIDPrefix, topolvm.LegacyPluginName),
					},
					VolumeHandle: "996a32d9-cc8e-472c-b5b3-6e16ce286bd8",
				},
			},
			NodeAffinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      topolvm.LegacyTopologyNodeKey,
									Operator: corev1.NodeSelectorOpIn,
									Values: []string{
										"topolvm-migrator-worker",
									},
								},
							},
						},
					},
				},
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			StorageClassName:              "topolvm-provisioner",
			VolumeMode:                    &mode,
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	}
}

var _ = Describe("test persistentvolume controller", func() {
	It("should migrate pv", func() {
		By("create a pv")
		pv := testPV()
		err := k8sClient.Create(testCtx, pv)
		Expect(err).ShouldNot(HaveOccurred())

		By("check a pv data")
		expectedAnnotations := map[string]string{
			AnnDynamicallyProvisioned: topolvm.PluginName,
		}
		expectedDriverName := topolvm.PluginName
		expectedProvisionerID := fmt.Sprintf("%s%s", testProvisonerIDPrefix, topolvm.PluginName)
		expectedNodeAffinityKey := topolvm.TopologyNodeKey
		expectedPolicy := corev1.PersistentVolumeReclaimDelete
		name := types.NamespacedName{
			Name: pv.Name,
		}
		Eventually(func() error {
			migratedPV := &corev1.PersistentVolume{}
			err := k8sClient.Get(testCtx, name, migratedPV)
			if err != nil {
				return fmt.Errorf("can not get target pv: err=%v", err)
			}
			if diff := cmp.Diff(expectedAnnotations, migratedPV.Annotations); diff != "" {
				return fmt.Errorf("annotations does not match: (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(expectedDriverName, migratedPV.Spec.PersistentVolumeSource.CSI.Driver); diff != "" {
				return fmt.Errorf("driver name does not match: (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(expectedProvisionerID, migratedPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes[provisionerIDKey]); diff != "" {
				return fmt.Errorf("provisionerID does not match: (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(expectedNodeAffinityKey, migratedPV.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Key); diff != "" {
				return fmt.Errorf("node affinity key does not match: (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(expectedPolicy, migratedPV.Spec.PersistentVolumeReclaimPolicy); diff != "" {
				return fmt.Errorf("reclaim policy does not match: (-want,+got):\n%s", diff)
			}
			return nil
		}).Should(Succeed())
	})
})
