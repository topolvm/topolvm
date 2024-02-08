package e2e

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	lvmdApp "github.com/topolvm/topolvm/cmd/lvmd/app"
	"github.com/topolvm/topolvm/internal/lvmd"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const nsMetricsTest = "metrics-test"

//go:embed testdata/metrics/pause-pod-with-pvc-template.yaml
var pausePodWithPVCTemplateYAML string

var metricsManifest = []byte(fmt.Sprintf(pausePodWithPVCTemplateYAML, nsMetricsTest, nsMetricsTest))

func testMetrics() {
	var cc CleanupContext
	BeforeEach(func() {
		createNamespace(nsMetricsTest)
		cc = commonBeforeEach()
	})
	AfterEach(func() {
		kubectl("delete", "namespaces/"+nsMetricsTest)
		commonAfterEach(cc)
	})

	Describe("topolvm-node", func() {
		It("should expose volume metrics", func() {
			By("creating a PVC and Pod")
			_, err := kubectlWithInput(metricsManifest, "apply", "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("waiting for the new Pod to be running")
			var nodeIP string
			Eventually(func(g Gomega) {
				var pod corev1.Pod
				err := getObjects(&pod, "pods", "-n", nsMetricsTest, "pause")
				g.Expect(err).ShouldNot(HaveOccurred())

				g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))

				nodeIP = pod.Status.HostIP
			}).Should(Succeed())

			By("parsing prometheus metrics")
			Eventually(func(g Gomega) {
				mfs, err := getMetricsFamily(nodeIP)
				g.Expect(err).ShouldNot(HaveOccurred())

				getGaugeValue := getGaugeValueWithLabels(map[string]string{
					"namespace":             nsMetricsTest,
					"persistentvolumeclaim": "topo-pvc",
				})

				mf := mfs["kubelet_volume_stats_capacity_bytes"]
				g.Expect(mf).NotTo(BeNil())
				g.Expect(getGaugeValue(mf)).NotTo(BeEquivalentTo(0))

				mf = mfs["kubelet_volume_stats_available_bytes"]
				g.Expect(mf).NotTo(BeNil())
				g.Expect(getGaugeValue(mf)).NotTo(BeEquivalentTo(0))
			}, 3*time.Minute).Should(Succeed())
		})

		It("should expose VG metrics", func() {
			By("reading metrics API endpoint")
			var pods corev1.PodList
			err := getObjects(&pods, "pods", "-n", "topolvm-system", "-l=app.kubernetes.io/component=node,app.kubernetes.io/name=topolvm")
			Expect(err).ShouldNot(HaveOccurred())

			pod := pods.Items[0]
			var mfs map[string]*dto.MetricFamily
			Eventually(func(g Gomega) {
				stdout, err := kubectl("exec", "-n", "topolvm-system", pod.Name, "-c=topolvm-node", "--", "curl", "http://localhost:8080/metrics")
				g.Expect(err).ShouldNot(HaveOccurred())

				var parser expfmt.TextParser
				mfs, err = parser.TextToMetricFamilies(bytes.NewReader(stdout))
				g.Expect(err).ShouldNot(HaveOccurred())
			}).Should(Succeed())

			By("loading LVMD config associating to the pod")
			deviceClasses, err := loadDeviceClasses(pod.Spec.NodeName)
			Expect(err).ShouldNot(HaveOccurred())

			By("checking topolvm_volumegroup_size_bytes metric exist")
			Expect(mfs).To(HaveKey("topolvm_volumegroup_size_bytes"))

			By("checking topolvm_volumegroup_available_bytes metric")
			mf := mfs["topolvm_volumegroup_available_bytes"]
			Expect(mf).NotTo(BeNil())

			Expect(mf.Metric).Should(HaveLen(len(deviceClasses)))

			for _, deviceClass := range deviceClasses {
				vgFree, err := vgFreeByte(deviceClass)
				Expect(err).ShouldNot(HaveOccurred())

				available := getGaugeValueWithLabels(map[string]string{
					"device_class": deviceClass.Name,
				})(mf)
				Expect(available).To(Equal(vgFree))
			}
		})
	})

	Describe("CSI sidecar of topolvm-node", func() {
		It("should open a port for metrics", func() {
			Eventually(func() error {
				_, err := kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "-c=topolvm-node", "--", "curl", "http://localhost:9808/metrics")
				return err
			}).Should(Succeed())
		})
	})

	Describe("CSI sidecar of topolvm-controller", func() {
		It("should open ports for metrics", func() {
			for _, port := range []string{"9808", "9809", "9810", "9811"} {
				Eventually(func() error {
					_, err := kubectl("exec", "-n", "topolvm-system", "deploy/topolvm-controller", "-c=topolvm-controller", "--", "curl", "http://localhost:"+port+"/metrics")
					return err
				}).Should(Succeed())
			}
		})
	})
}

func getMetricsFamily(nodeIP string) (map[string]*dto.MetricFamily, error) {
	resp, err := http.Get("http://" + nodeIP + ":10255/metrics")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parser expfmt.TextParser
	return parser.TextToMetricFamilies(resp.Body)
}

func getGaugeValueWithLabels(labels map[string]string) func(*dto.MetricFamily) int64 {
	return func(mf *dto.MetricFamily) int64 {
		for _, m := range mf.Metric {
			if !haveLabels(m, labels) {
				continue
			}
			if m.Gauge == nil {
				return 0
			}
			if m.Gauge.Value == nil {
				return 0
			}
			return int64(*m.Gauge.Value)
		}
		return 0
	}
}

func haveLabels(m *dto.Metric, labels map[string]string) bool {
OUTER:
	for k, v := range labels {
		for _, label := range m.Label {
			if k == *label.Name && v == *label.Value {
				continue OUTER
			}
		}
		return false
	}
	return true
}

func loadDeviceClasses(node string) ([]*lvmd.DeviceClass, error) {
	var data []byte

	var cm corev1.ConfigMap
	err := getObjects(&cm, "cm", "-n", "topolvm-system", "topolvm-lvmd-0")
	if err == nil {
		data = []byte(cm.Data["lvmd.yaml"])
	} else if err != ErrObjectNotFound {
		return nil, err
	} else {
		path, ok := map[string]string{
			"topolvm-e2e-worker":  "lvmd1.yaml",
			"topolvm-e2e-worker2": "lvmd2.yaml",
			"topolvm-e2e-worker3": "lvmd3.yaml",
		}[node]
		if !ok {
			return nil, fmt.Errorf("unknown node: %s", node)
		}
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}

	var config lvmdApp.Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return config.DeviceClasses, nil
}

func vgFreeByte(deviceClass *lvmd.DeviceClass) (int64, error) {
	output, err := execAtLocal("sudo", nil, "vgs",
		"-o", "vg_free",
		"--noheadings",
		"--units=b",
		"--nosuffix",
		deviceClass.VolumeGroup,
	)
	if err != nil {
		return 0, err
	}

	free, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, err
	}

	// FIXME? the current implementation does not subtract spare-gb if the type is thin.
	if deviceClass.Type == lvmd.TypeThin {
		return int64(free), nil
	}

	spare := int(deviceClass.GetSpare())
	if free < spare {
		free = 0
	} else {
		free -= spare
	}
	return int64(free), nil
}
