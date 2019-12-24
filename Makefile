
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
export GO111MODULE=on
export GOPROXY=https://proxy.golang.org

SHELL := /bin/bash -o pipefail
VERSION_PACKAGE = github.com/replicatedhq/troubleshoot/pkg/version
VERSION ?=`git describe --tags --dirty`
DATE=`date -u +"%Y-%m-%dT%H:%M:%SZ"`

GIT_TREE = $(shell git rev-parse --is-inside-work-tree 2>/dev/null)
ifneq "$(GIT_TREE)" ""
define GIT_UPDATE_INDEX_CMD
git update-index --assume-unchanged
endef
define GIT_SHA
`git rev-parse HEAD`
endef
else
define GIT_UPDATE_INDEX_CMD
echo "Not a git repo, skipping git update-index"
endef
define GIT_SHA
""
endef
endif

define LDFLAGS
-ldflags "\
	-X ${VERSION_PACKAGE}.version=${VERSION} \
	-X ${VERSION_PACKAGE}.gitSHA=${GIT_SHA} \
	-X ${VERSION_PACKAGE}.buildTime=${DATE} \
"
endef

all: test

.PHONY: ffi
ffi: fmt vet
	go build ${LDFLAGS} -o bin/troubleshoot.so -buildmode=c-shared ffi/main.go

# Run tests
test: generate fmt vet
	go test ./pkg/... ./cmd/... -coverprofile cover.out

.PHONY: support-bundle
support-bundle: generate fmt vet
	go build ${LDFLAGS} -o bin/support-bundle github.com/replicatedhq/troubleshoot/cmd/troubleshoot

.PHONY: preflight
preflight: generate fmt vet
	go build ${LDFLAGS} -o bin/preflight github.com/replicatedhq/troubleshoot/cmd/preflight

.PHONY: analyze
analyze: generate fmt vet
	go build ${LDFLAGS} -o bin/analyze github.com/replicatedhq/troubleshoot/cmd/analyze

.PHONY: fmt
fmt:
	go fmt ./pkg/... ./cmd/...

.PHONY: vet
vet:
	go vet ./pkg/... ./cmd/...

.PHONY: generate
generate: controller-gen client-gen
	controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./pkg/apis/...
	client-gen --output-package=github.com/replicatedhq/troubleshoot/pkg/client --clientset-name troubleshootclientset --input-base github.com/replicatedhq/troubleshoot/pkg/apis --input troubleshoot/v1beta1 -h ./hack/boilerplate.go.txt

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.4
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

.PHONY: release
release: export GITHUB_TOKEN = $(shell echo ${GITHUB_TOKEN_TROUBLESHOOT})
release:
	curl -sL https://git.io/goreleaser | bash -s -- --rm-dist --config deploy/.goreleaser.yml

.PHONY: snapshot-release
snapshot-release:
	curl -sL https://git.io/goreleaser | bash -s -- --rm-dist --snapshot --config deploy/.goreleaser.snapshot.yml
	docker push replicated/troubleshoot:alpha
	docker push replicated/preflight:alpha

.PHONY: local-release
local-release:
	curl -sL https://git.io/goreleaser | bash -s -- --rm-dist --snapshot --config deploy/.goreleaser.local.yml
	docker tag replicated/troubleshoot:alpha localhost:32000/troubleshoot:alpha
	docker tag replicated/preflight:alpha localhost:32000/preflight:alpha
	docker push localhost:32000/troubleshoot:alpha
	docker push localhost:32000/preflight:alpha

.PHONY: run-preflight
run-preflight: preflight
	./bin/preflight ./examples/preflight/sample-preflight.yaml

.PHONY: run-troubleshoot
run-troubleshoot: support-bundle
	./bin/support-bundle ./examples/troubleshoot/sample-troubleshoot.yaml

.PHONY: run-analyze
run-analyze: analyze
	./bin/analyze --analyzers ./examples/troubleshoot/sample-analyzers.yaml ./support-bundle.tar.gz
