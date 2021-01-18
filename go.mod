module github.com/topolvm/topolvm

go 1.13

replace launchpad.net/gocheck => github.com/go-check/check v0.0.0-20180628173108-788fd7840127

require (
	cloud.google.com/go v0.63.0 // indirect
	github.com/bazelbuild/bazel-gazelle v0.19.1-0.20191105222053-70208cbdc798 // indirect
	github.com/container-storage-interface/spec v1.1.0
	github.com/coredns/corefile-migration v1.0.4 // indirect
	github.com/cybozu-go/log v1.5.0
	github.com/cybozu-go/well v1.10.0
	github.com/docker/libnetwork v0.8.0-dev.2.0.20190624125649-f0e46a78ea34 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/golang/protobuf v1.4.2
	github.com/google/cadvisor v0.35.0 // indirect
	github.com/google/go-cmp v0.5.2
	github.com/kubernetes-csi/csi-test v2.2.0+incompatible // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.6.0
	github.com/pseudomuto/protoc-gen-doc v1.3.2
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.1
	github.com/vishvananda/netlink v1.0.0 // indirect
	golang.org/x/sys v0.0.0-20200922070232-aee5d888a860
	google.golang.org/grpc v1.32.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.0.0
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.18.9
	k8s.io/apimachinery v0.18.9
	k8s.io/client-go v0.18.9
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.17.0-beta.1
	k8s.io/repo-infra v0.0.1-alpha.1 // indirect
	k8s.io/system-validators v1.0.4 // indirect
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/controller-tools v0.4.0
	sigs.k8s.io/yaml v1.2.0
)

replace (
	k8s.io/api => k8s.io/api v0.18.9
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.9
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.9
	k8s.io/apiserver => k8s.io/apiserver v0.18.9
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.9
	k8s.io/client-go => k8s.io/client-go v0.18.9
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.9
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.9
	k8s.io/code-generator => k8s.io/code-generator v0.18.9
	k8s.io/component-base => k8s.io/component-base v0.18.9
	k8s.io/cri-api => k8s.io/cri-api v0.18.9
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.9
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.9
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.9
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.9
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.9
	k8s.io/kubectl => k8s.io/kubectl v0.18.9
	k8s.io/kubelet => k8s.io/kubelet v0.18.9
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.9
	k8s.io/metrics => k8s.io/metrics v0.18.9
	k8s.io/node-api => k8s.io/node-api v0.18.9
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.9
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.18.9
	k8s.io/sample-controller => k8s.io/sample-controller v0.18.9
)
