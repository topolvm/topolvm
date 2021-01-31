module github.com/topolvm/topolvm

go 1.13

replace launchpad.net/gocheck => github.com/go-check/check v0.0.0-20180628173108-788fd7840127

require (
	cloud.google.com/go v0.63.0 // indirect
	github.com/container-storage-interface/spec v1.3.0
	github.com/cybozu-go/log v1.5.0
	github.com/cybozu-go/well v1.10.0
	github.com/go-logr/logr v0.3.0
	github.com/go-logr/zapr v0.2.0 // indirect
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.2
	github.com/imdario/mergo v0.3.10 // indirect
	github.com/kubernetes-csi/csi-test/v4 v4.0.2
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.10.0
	github.com/pseudomuto/protoc-gen-doc v1.3.2
	github.com/spf13/cobra v1.1.1
	github.com/spf13/viper v1.7.1
	go.uber.org/zap v1.15.0 // indirect
	golang.org/x/sys v0.0.0-20201112073958-5cba982894dd
	gomodules.xyz/jsonpatch/v2 v2.1.0 // indirect
	google.golang.org/grpc v1.32.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.0.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	k8s.io/api v0.20.2
	k8s.io/apiextensions-apiserver v0.20.2 // indirect
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/klog v1.0.0
	k8s.io/mount-utils v0.20.2
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	sigs.k8s.io/controller-runtime v0.6.5
	sigs.k8s.io/controller-tools v0.4.0
	sigs.k8s.io/yaml v1.2.0
)
