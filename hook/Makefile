
GOFLAGS = -mod=vendor
export GOFLAGS

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

all: test

# Run tests
test: generate fmt vet manifests
	go test ./api/... ./controllers/... -coverprofile cover.out

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crd/bases
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	controller-gen $(CRD_OPTIONS) rbac:roleName=topolvm-system:topolvm-hook webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: 
	controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./api/...
