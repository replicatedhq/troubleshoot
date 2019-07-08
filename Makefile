
# Image URL to use all building/pushing image targets
IMG ?= controller:latest

all: test manager

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

.PHONY: manager
manager: generate fmt vet
	go build -o bin/manager github.com/replicatedhq/troubleshoot/cmd/manager

.PHONY: troubleshoot
troubleshoot: generate fmt vet
	go build -o bin/troubleshoot github.com/replicatedhq/troubleshoot/cmd/troubleshoot

.PHONY: collector
collector: generate fmt vet
	go build -o bin/collector github.com/replicatedhq/troubleshoot/cmd/collector

.PHONY: preflight
preflight: generate fmt vet
	go build -o bin/preflight github.com/replicatedhq/troubleshoot/cmd/preflight

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crd

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crd
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	controller-gen paths=./pkg/apis/...

.PHONY: fmt
fmt:
	go fmt ./pkg/... ./cmd/...

.PHONY: vet
vet:
	go vet ./pkg/... ./cmd/...

.PHONY: generate
generate: controller-gen # client-gen
	controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./pkg/apis/...
	# client-gen --output-package=github.com/replicatedhq/troubleshoot/pkg/client --clientset-name troubleshootclientset --input-base github.com/replicatedhq/troubleshoot/pkg/apis --input troubleshoot/v1beta1 -h ./hack/boilerplate.go.txt

# Build the docker image
docker-build: test
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.0-beta.2
CONTROLLER_GEN=$(shell go env GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# find or download client-gen
client-gen:
ifeq (, $(shell which client-gen))
	go get k8s.io/code-generator/cmd/client-gen@kubernetes-1.13.5
CLIENT_GEN=$(shell go env GOPATH)/bin/client-gen
else
CLIENT_GEN=$(shell which client-gen)
endif
