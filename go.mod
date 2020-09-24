module github.com/topolvm/topolvm

go 1.13

replace launchpad.net/gocheck => github.com/go-check/check v0.0.0-20180628173108-788fd7840127

require (
	cloud.google.com/go v0.63.0 // indirect
	github.com/container-storage-interface/spec v1.1.0
	github.com/cybozu-go/log v1.5.0
	github.com/cybozu-go/well v1.10.0
	github.com/go-logr/logr v0.1.0
	github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp v0.5.2
	github.com/kubernetes-csi/csi-test v2.2.0+incompatible
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.6.0
	github.com/pseudomuto/protoc-gen-doc v1.3.2
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.1
	golang.org/x/sys v0.0.0-20200922070232-aee5d888a860
	google.golang.org/grpc v1.32.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v0.0.0-20200812184716-7d8921505e1b
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.18.9
	k8s.io/apimachinery v0.18.9
	k8s.io/client-go v0.18.9
	k8s.io/klog v1.0.0
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/controller-tools v0.4.0
	sigs.k8s.io/yaml v1.2.0
)
