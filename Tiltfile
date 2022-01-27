allow_k8s_contexts('kind-topolvm-e2e')

def get_all_go_files():
  return str(local("find -type f -name '*.go' -not -name '*_test.go'")).split("\n")

all_go_files = get_all_go_files()
ignores=["e2e/tmpbin", "e2e/bin", "e2e/build", "build", "bin", "include", "testbin", "e2e/topolvm.img", "*/.docker_temp_*"]

local_resource("hypertopolvm", "make -C e2e topolvm.img", deps=all_go_files, ignore=ignores)

docker_build("topolvm:dev", "e2e/tmpbin", dockerfile="e2e/Dockerfile")

load("ext://cert_manager", "deploy_cert_manager")
deploy_cert_manager()

load('ext://namespace', 'namespace_create')
namespace_create(
  'topolvm-system',
  labels=['topolvm.cybozu.com/webhook: ignore']
)

local("kubectl label namespace kube-system topolvm.cybozu.com/webhook=ignore --overwrite")

k8s_yaml(
  helm("./charts/topolvm",
    namespace="topolvm-system",
    name="topolvm",
    values=["e2e/manifests/values/deployment-scheduler.yaml"],
    set=["cert-manager.enabled=false"]
  )
)
