module github.com/cybozu-go/topolvm

go 1.12

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190805141119-fdd30b57c827
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190805141520-2fe0317bcee0
	launchpad.net/gocheck => github.com/go-check/check v0.0.0-20180628173108-788fd7840127
)

require (
	github.com/GeertJohan/go.rice v1.0.0 // indirect
	github.com/cloudflare/cfssl v0.0.0-20190510060611-9c027c93ba9e
	github.com/container-storage-interface/spec v1.1.0
	github.com/cybozu-go/log v1.5.0
	github.com/cybozu-go/well v1.8.1
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/go-sql-driver/mysql v1.4.1 // indirect
	github.com/golang/protobuf v1.3.1
	github.com/google/certificate-transparency-go v1.0.21 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jmhodges/clock v0.0.0-20160418191101-880ee4c33548 // indirect
	github.com/jmoiron/sqlx v1.2.0 // indirect
	github.com/json-iterator/go v1.1.6 // indirect
	github.com/kisielk/sqlstruct v0.0.0-20150923205031-648daed35d49 // indirect
	github.com/kubernetes-csi/csi-test v2.0.1+incompatible
	github.com/lib/pq v1.1.1 // indirect
	github.com/mattn/go-sqlite3 v1.10.0 // indirect
	github.com/onsi/ginkgo v1.6.0
	github.com/onsi/gomega v1.4.2
	github.com/spf13/cobra v0.0.3
	github.com/spf13/viper v1.2.1
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/zap v1.10.0 // indirect
	golang.org/x/net v0.0.0-20190311183353-d8887717615a
	golang.org/x/sys v0.0.0-20190312061237-fead79001313
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/grpc v1.23.0
	k8s.io/api v0.0.0-20190805141119-fdd30b57c827
	k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/klog v0.3.1
	sigs.k8s.io/controller-runtime v0.2.0-beta.5
)
