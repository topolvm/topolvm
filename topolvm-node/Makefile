
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"
GOFLAGS = -mod=vendor
export GOFLAGS

all: manager

# Run tests
test: generate fmt vet manifests
	go test ./api/... ./controllers/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./main.go --node-name=localhost

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crd/bases

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crd/bases
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	controller-gen $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./api/...;./controllers/..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	test -z "$$(go vet ./... | grep -v '^vendor' | tee /dev/stderr)"

# Generate code
generate:
	controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./api/...
