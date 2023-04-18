package e2e

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/topolvm/topolvm/lvmd"
	"github.com/topolvm/topolvm/pkg/lvmd/cmd"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func testNode() {
	var cc CleanupContext
	BeforeEach(func() {
		cc = commonBeforeEach()
	})
	AfterEach(func() {
		commonAfterEach(cc)
	})

	It("should expose Prometheus metrics", func() {
		By("reading metrics API endpoint")
		var pods corev1.PodList
		err := getObjects(&pods, "pods", "-n", "topolvm-system", "-l=app.kubernetes.io/component=node,app.kubernetes.io/name=topolvm")
		Expect(err).ShouldNot(HaveOccurred())

		pod := pods.Items[0]
		var metricFamilies map[string]*dto.MetricFamily
		Eventually(func() error {
			stdout, err := kubectl("exec", "-n", "topolvm-system", pod.Name, "-c=topolvm-node", "--", "curl", "http://localhost:8080/metrics")
			if err != nil {
				return err
			}
			var parser expfmt.TextParser
			metricFamilies, err = parser.TextToMetricFamilies(bytes.NewReader(stdout))
			return err
		}).Should(Succeed())

		By("loading LVMD config associating to the pod")
		deviceClasses, err := loadDeviceClasses(pod.Spec.NodeName)
		Expect(err).ShouldNot(HaveOccurred())

		By("checking topolvm_volumegroup_size_bytes metric exist")
		Expect(metricFamilies).To(HaveKey("topolvm_volumegroup_size_bytes"))

		By("checking topolvm_volumegroup_available_bytes metric")
		family := metricFamilies["topolvm_volumegroup_available_bytes"]
		Expect(family).NotTo(BeNil())

		Expect(family.Metric).Should(HaveLen(len(deviceClasses)))

		for _, deviceClass := range deviceClasses {
			vgFree, err := vgFreeByte(deviceClass)
			Expect(err).ShouldNot(HaveOccurred())

			var metric *dto.Metric
			Expect(family.Metric).To(ContainElement(HaveField("Label", WithTransform(getDeviceClassName, Equal(deviceClass.Name))), &metric))
			Expect(metric.Gauge.Value).To(HaveValue(BeEquivalentTo(vgFree)))
		}
	})

	Describe("CSI sidecar of topolvm-node", func() {
		It("should open a port for metrics", func() {
			Eventually(func() error {
				_, err := kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "-c=topolvm-node", "--", "curl", "http://localhost:9808/metrics")
				return err
			}).Should(Succeed())
		})
	})
}

func getDeviceClassName(label []*dto.LabelPair) string {
	for _, label := range label {
		if *label.Name == "device_class" {
			return *label.Value
		}
	}
	return ""
}

func loadDeviceClasses(node string) ([]*lvmd.DeviceClass, error) {
	var data []byte
	if isDaemonsetLvmdEnvSet() {
		var cm corev1.ConfigMap
		err := getObjects(&cm, "cm", "-n", "topolvm-system", "topolvm-lvmd-0")
		if err != nil {
			return nil, err
		}
		data = []byte(cm.Data["lvmd.yaml"])
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

	var config cmd.Config
	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return config.DeviceClasses, nil
}

func vgFreeByte(deviceClass *lvmd.DeviceClass) (int, error) {
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
		return free, nil
	}

	spare := int(deviceClass.GetSpare())
	if free < spare {
		free = 0
	} else {
		free -= spare
	}
	return free, nil
}
