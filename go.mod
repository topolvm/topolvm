module github.com/topolvm/topolvm

go 1.13

replace launchpad.net/gocheck => github.com/go-check/check v0.0.0-20180628173108-788fd7840127

require (
	github.com/GeertJohan/go.rice v1.0.0 // indirect
	github.com/cloudflare/cfssl v0.0.0-20190510060611-9c027c93ba9e
	github.com/container-storage-interface/spec v1.1.0
	github.com/cybozu-go/log v1.5.0
	github.com/cybozu-go/well v1.10.0
	github.com/go-logr/logr v0.1.0
	github.com/go-sql-driver/mysql v1.5.0 // indirect
	github.com/golang/protobuf v1.4.2
	github.com/google/certificate-transparency-go v1.1.0 // indirect
	github.com/google/go-cmp v0.5.2
	github.com/jmhodges/clock v0.0.0-20160418191101-880ee4c33548 // indirect
	github.com/jmoiron/sqlx v1.2.0 // indirect
	github.com/kisielk/sqlstruct v0.0.0-20150923205031-648daed35d49 // indirect
	github.com/kubernetes-csi/csi-test v2.2.0+incompatible
	github.com/lib/pq v1.7.0 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.6.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.3.2
	golang.org/x/sys v0.0.0-20200922070232-aee5d888a860
	google.golang.org/grpc v1.28.0
	k8s.io/api v0.18.9
	k8s.io/apimachinery v0.18.9
	k8s.io/client-go v0.18.9
	k8s.io/klog v1.0.0
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/yaml v1.2.0
)
