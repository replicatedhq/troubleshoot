
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

.PHONY: run
run: generate fmt vet
	TROUBLESHOOT_EXTERNAL_MANAGER=1 go run ./cmd/manager/main.go

.PHONY: install
install: manifests
	kubectl apply -f config/crds

.PHONY: deploy
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

.PHONY: manifests
manifests:
	controller-gen paths=./pkg/apis/... output:dir=./config/crds

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

.PHONY: snapshot-release
snapshot-release:
	curl -sL https://git.io/goreleaser | bash -s -- --rm-dist --snapshot --config deploy/.goreleaser.snapshot.yml

.PHONY: local-release
local-release: snapshot-release
	docker tag replicated/troubleshoot:alpha localhost:32000/troubleshoot:alpha
	docker tag replicated/preflight:alpha localhost:32000/preflight:alpha
	docker tag replicated/troubleshoot-manager:alpha localhost:32000/troubleshoot-manager:alpha
	docker push localhost:32000/troubleshoot:alpha
	docker push localhost:32000/preflight:alpha
	docker push localhost:32000/troubleshoot-manager:alpha

.PHONY: run-preflight
run-preflight: preflight
	./bin/preflight run \
		--collector-image=localhost:32000/troubleshoot:alpha \
		--collector-pullpolicy=Always \
		--image=localhost:32000/troubleshoot:alpha \
		--pullpolicy=Always
