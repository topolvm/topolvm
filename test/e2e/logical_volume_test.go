package e2e

import (
	"context"
	_ "embed"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	clientwrapper "github.com/topolvm/topolvm/internal/client"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const nsLogicalVolumeTest = "logical-volume"

//go:embed testdata/logical_volume/pvc-template.yaml
var pvcTemplateYAMLForLV string

func testLogicalVolume() {
	var cc CleanupContext
	BeforeEach(func() {
		createNamespace(nsLogicalVolumeTest)
		cc = commonBeforeEach()
	})
	AfterEach(func() {
		// When a test fails, I want to investigate the cause. So please don't remove the namespace!
		if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
			_, err := kubectl("delete", "namespaces", nsLogicalVolumeTest)
			Expect(err).ShouldNot(HaveOccurred())
		}

		commonAfterEach(cc)
	})

	k8sClient := createK8sClient()

	It("should set Status.CurrentSize", func() {
		pvcName := "check-current-size"
		pvcYaml := fmt.Sprintf(pvcTemplateYAMLForLV, pvcName)
		_, err := kubectlWithInput([]byte(pvcYaml), "apply", "-n", nsLogicalVolumeTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("checking that Status.CurrentSize exists")
		var pvc corev1.PersistentVolumeClaim
		Eventually(func(g Gomega) {
			g.Expect(getObjects(&pvc, "pvc", "-n", nsLogicalVolumeTest, pvcName)).Should(Succeed())
			g.Expect(pvc.Spec.VolumeName).NotTo(BeEmpty())
		}).Should(Succeed())
		lv, err := getLogicalVolume(pvc.Spec.VolumeName)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(lv.Status.CurrentSize).NotTo(BeNil())
		oldCurrentSize := lv.Status.CurrentSize.Value()

		var ds appsv1.DaemonSet
		err = getObjects(&ds, "daemonset", "-n", "topolvm-system", "topolvm-node")
		Expect(err).ShouldNot(HaveOccurred())
		desiredTopoLVMNodeCount := int(ds.Status.DesiredNumberScheduled)

		By("clearing Status.CurrentSize")
		stopTopoLVMNode(lv.Spec.NodeName, desiredTopoLVMNodeCount-1)
		clearCurrentSize(k8sClient, lv.Name)
		// sanity check for clearing CurrentSize
		lv, err = getLogicalVolume(lv.Name)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(lv.Status.CurrentSize).To(BeNil())

		By("checking that Status.CurrentSize is set to previous value if it is missing and spec.Size is not modified")
		startTopoLVMNode(desiredTopoLVMNodeCount)
		currentSize := waitForSettingCurrentSize(lv.Name)
		Expect(currentSize).To(BeEquivalentTo(oldCurrentSize))

		By("clearing Status.CurrentSize and changing Spec.Size to 2Gi")
		stopTopoLVMNode(lv.Spec.NodeName, desiredTopoLVMNodeCount-1)
		clearCurrentSize(k8sClient, lv.Name)
		_, err = kubectl("patch", "logicalvolumes", lv.Name, "--type=json", "-p",
			`[{"op": "replace", "path": "/spec/size", "value": "2Gi"}]`)
		Expect(err).ShouldNot(HaveOccurred())
		// sanity check for clearing CurrentSize
		lv, err = getLogicalVolume(lv.Name)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(lv.Status.CurrentSize).To(BeNil())

		By("checking that Status.CurrentSize is set to resized size if it is missing and spec.Size is modified")
		startTopoLVMNode(desiredTopoLVMNodeCount)
		currentSize = waitForSettingCurrentSize(lv.Name)
		Expect(currentSize).To(BeEquivalentTo(int64(2 * 1024 * 1024 * 1024)))

		By("checking actual volume size is changed to 2Gi")
		lvInfo, err := getLVInfo(lv.Status.VolumeID)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(lvInfo.size).To(BeEquivalentTo(2 * 1024 * 1024 * 1024))
	})
}

func createK8sClient() client.Client {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, nil)
	config, err := kubeConfig.ClientConfig()
	Expect(err).ShouldNot(HaveOccurred())
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(topolvmv1.AddToScheme(scheme))
	utilruntime.Must(topolvmlegacyv1.AddToScheme(scheme))
	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	Expect(err).ShouldNot(HaveOccurred())
	return clientwrapper.NewWrappedClient(k8sClient)
}

func clearCurrentSize(c client.Client, lvName string) {
	// kubectl v1.23 patch subcommand does not support --subresource option, so use api client.
	ctx := context.Background()
	var lv topolvmv1.LogicalVolume
	Expect(c.Get(ctx, client.ObjectKey{Name: lvName}, &lv)).Should(Succeed())

	lv.Status.CurrentSize = nil

	Expect(c.Status().Update(ctx, &lv)).Should(Succeed())
}

func waitForSettingCurrentSize(lvName string) int64 {
	var lv *topolvmv1.LogicalVolume
	Eventually(func(g Gomega) {
		var err error
		lv, err = getLogicalVolume(lvName)
		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(lv.Status.CurrentSize).NotTo(BeNil())
	}).Should(Succeed())
	return lv.Status.CurrentSize.Value()
}

func getLogicalVolume(lvName string) (*topolvmv1.LogicalVolume, error) {
	var lv topolvmv1.LogicalVolume
	err := getObjects(&lv, "logicalvolumes", lvName)
	return &lv, err
}

func waitForTopoLVMNodeDSPatched(patch string, patchType string, desiredTopoLVMNodeCount int) {
	_, err := kubectl("patch", "-n", "topolvm-system", "daemonset", "topolvm-node", "--type", patchType, "-p", patch)
	Expect(err).ShouldNot(HaveOccurred())

	Eventually(func(g Gomega) {
		var pods corev1.PodList
		err := getObjects(&pods, "pod", "-n", "topolvm-system", "-l", "app.kubernetes.io/component=node")
		if desiredTopoLVMNodeCount == 0 {
			g.Expect(err).To(BeIdenticalTo(ErrObjectNotFound))
		} else {
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(pods.Items).To(HaveLen(desiredTopoLVMNodeCount))
		}
	}).Should(Succeed())
}

func startTopoLVMNode(desiredTopoLVMNodeCount int) {
	waitForTopoLVMNodeDSPatched(
		`[{"op": "remove", "path": "/spec/template/spec/affinity"}]`,
		"json",
		desiredTopoLVMNodeCount,
	)
}

func stopTopoLVMNode(nodeName string, desiredTopoLVMNodeCount int) {
	patch := fmt.Sprintf(`
				{
					"spec": {
						"template": {
							"spec": {
								"affinity": {
									"nodeAffinity": {
										"requiredDuringSchedulingIgnoredDuringExecution": {
											"nodeSelectorTerms": [
												{
													"matchFields": [
														{
															"key": "metadata.name",
															"operator": "NotIn",
															"values": ["%s"]
														}
													]
												}
											]
										}
									}
								}
							}
						}
					}
				}
			`, nodeName)
	waitForTopoLVMNodeDSPatched(patch, "strategic", desiredTopoLVMNodeCount)
}
