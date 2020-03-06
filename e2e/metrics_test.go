package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	corev1 "k8s.io/api/core/v1"
)

const nsMetricsTest = "metrics-test"

var metricsManifest = []byte(fmt.Sprintf(`
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: topo-pvc
  namespace: %s
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm-provisioner
---
apiVersion: v1
kind: Pod
metadata:
  name: ubuntu
  namespace: %s
  labels:
    app.kubernetes.io/name: ubuntu
spec:
  containers:
    - name: ubuntu
      image: quay.io/cybozu/ubuntu:18.04
      command: ["/usr/local/bin/pause"]
      volumeMounts:
        - mountPath: /test1
          name: my-volume
  volumes:
    - name: my-volume
      persistentVolumeClaim:
        claimName: topo-pvc
`, nsMetricsTest, nsMetricsTest))

func testMetrics() {
	BeforeEach(func() {
		createNamespace(nsMetricsTest)
	})
	AfterEach(func() {
		kubectl("delete", "namespaces/"+nsMetricsTest)
	})

	It("should export volume metrics", func() {
		By("creating a PVC and Pod")
		_, _, err := kubectlWithInput(metricsManifest, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("waiting for the new Pod to be running")
		var nodeIP string
		Eventually(func() error {
			stdout, _, err := kubectl("-n", nsMetricsTest, "get", "pods", "ubuntu", "-o", "json")
			if err != nil {
				return err
			}
			var pod corev1.Pod
			err = json.Unmarshal(stdout, &pod)
			if err != nil {
				return err
			}
			if pod.Status.Phase != corev1.PodRunning {
				return errors.New("pod is not running")
			}
			nodeIP = pod.Status.HostIP
			return nil
		}).Should(Succeed())

		By("parsing prometheus metrics")
		Eventually(func() error {
			mfs, err := getMetricsFamily(nodeIP)
			if err != nil {
				return err
			}

			mf, ok := mfs["kubelet_volume_stats_capacity_bytes"]
			if !ok {
				return errors.New("no kubelet_volume_stats_capacity_bytes metrics family")
			}
			capacity := getGaugeValue("topo-pvc", mf)
			if capacity == 0 {
				return errors.New("no volume capacity bytes")
			}

			mf, ok = mfs["kubelet_volume_stats_available_bytes"]
			if !ok {
				return errors.New("no kubelet_volume_stats_available_bytes metrics family")
			}
			available := getGaugeValue("topo-pvc", mf)
			if available == 0 {
				return errors.New("no volume available bytes")
			}
			return nil
		}, 3*time.Minute).Should(Succeed())
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

func getGaugeValue(pvc string, mf *dto.MetricFamily) int64 {
	for _, m := range mf.Metric {
		labels := map[string]string{
			"namespace":             nsMetricsTest,
			"persistentvolumeclaim": pvc,
		}
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
