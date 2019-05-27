module github.com/cybozu-go/topolvm/topolvm-node

go 1.12

require (
	github.com/cybozu-go/topolvm v0.0.0-20190523145556-8be767bb61b2
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.6.0
	github.com/onsi/gomega v1.4.2
	github.com/spf13/cobra v0.0.4
	github.com/spf13/viper v1.3.2
	golang.org/x/net v0.0.0-20190311183353-d8887717615a
	google.golang.org/grpc v1.21.0
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	sigs.k8s.io/controller-runtime v0.2.0-beta.1
)
