
# Image URL to use all building/pushing image targets
IMG ?= controller:latest

SHELL := bash -o pipefail
VERSION_PACKAGE = github.com/replicatedhq/troubleshoot/pkg/version
VERSION ?=`git describe --tags --dirty`
DATE=`date -u +"%Y-%m-%dT%H:%M:%SZ"`
RUN?=""

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
	-s -w \
	-X ${VERSION_PACKAGE}.version=${VERSION} \
	-X ${VERSION_PACKAGE}.gitSHA=${GIT_SHA} \
	-X ${VERSION_PACKAGE}.buildTime=${DATE} \
"
endef

BUILDTAGS = "netgo containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp"
BUILDFLAGS = -tags ${BUILDTAGS} -installsuffix netgo
BUILDPATHS = ./pkg/... ./cmd/... ./internal/...
E2EPATHS = ./test/e2e/...
TESTFLAGS ?= -v -coverprofile cover.out

.DEFAULT_GOAL := all
all: clean build test

.PHONY: ffi
ffi: fmt vet
	go build ${BUILDFLAGS} ${LDFLAGS} -o bin/troubleshoot.so -buildmode=c-shared ffi/main.go

.PHONY: test
test: generate fmt vet
	if [ -n $(RUN) ]; then \
		go test ${BUILDFLAGS} ${BUILDPATHS} ${TESTFLAGS} -run $(RUN); \
	else \
		go test ${BUILDFLAGS} ${BUILDPATHS} ${TESTFLAGS}; \
	fi

# Go tests that require a K8s instance
# TODOLATER: merge with test, so we get unified coverage reports? it'll add 21~sec to the test job though...
.PHONY: test-integration
test-integration: generate fmt vet
	go test -v --tags="integration exclude_graphdriver_devicemapper exclude_graphdriver_btrfs" ${BUILDPATHS}

.PHONY: preflight-e2e-test
preflight-e2e-test:
	./test/validate-preflight-e2e.sh

.PHONY: run-examples
run-examples:
	./test/run-examples.sh

.PHONY: support-bundle-e2e-test
support-bundle-e2e-test:
	./test/validate-support-bundle-e2e.sh

.PHONY: support-bundle-e2e-go-test
support-bundle-e2e-go-test:
	if [ -n $(RUN) ]; then \
		go test ${BUILDFLAGS} ${E2EPATHS} -v -run $(RUN); \
	else \
		go test ${BUILDFLAGS} ${E2EPATHS} -v; \
	fi

rebuild: clean build

# Build all binaries in parallel ( -j )
build: tidy
	@echo "Build cli binaries"
	$(MAKE) -j bin/support-bundle bin/preflight bin/analyze bin/collect

.PHONY: clean
clean:
	@rm -f bin/analyze
	@rm -f bin/support-bundle
	@rm -f bin/collect
	@rm -f bin/preflight
	@rm -f bin/troubleshoot.h
	@rm -f bin/troubleshoot.so
	@rm -f bin/schemagen
	@rm -f bin/docsgen

.PHONY: tidy
tidy:
	go mod tidy

# Prints  the diff of the changes that would be made by `go mod tidy`. Used in CI
.PHONY: tidy-diff
tidy-diff:
	go mod tidy -diff

# Only build when any of the files in SOURCES changes, or if bin/<file> is absent
MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
SOURCES := $(shell find $(MAKEFILE_DIR) -type f \( -name "*.go" -o -name "go.mod" -o -name "go.sum" \))
bin/support-bundle: $(SOURCES)
	go build ${BUILDFLAGS} ${LDFLAGS} -o bin/support-bundle github.com/replicatedhq/troubleshoot/cmd/troubleshoot

bin/preflight: $(SOURCES)
	go build ${BUILDFLAGS} ${LDFLAGS} -o bin/preflight github.com/replicatedhq/troubleshoot/cmd/preflight

bin/analyze: $(SOURCES)
	go build ${BUILDFLAGS} ${LDFLAGS} -o bin/analyze github.com/replicatedhq/troubleshoot/cmd/analyze

bin/collect: $(SOURCES)
	go build ${BUILDFLAGS} ${LDFLAGS} -o bin/collect github.com/replicatedhq/troubleshoot/cmd/collect

.PHONY: support-bundle
support-bundle: bin/support-bundle

.PHONY: preflight
preflight: bin/preflight

.PHONY: analyze
analyze: bin/analyze

.PHONY: collect
collect: bin/collect

build-linux: tidy
	@echo "Build cli binaries for Linux"
	GOOS=linux GOARCH=amd64 $(MAKE) -j bin/support-bundle bin/preflight bin/analyze bin/collect

.PHONY: fmt
fmt:
	go fmt ${BUILDPATHS}

.PHONY: vet
vet:
	go vet ${BUILDFLAGS} ${BUILDPATHS}

.PHONY: generate
generate: controller-gen client-gen
	$(CONTROLLER_GEN) \
		object:headerFile=./hack/boilerplate.go.txt paths=./pkg/apis/...
	$(CLIENT_GEN) \
		--output-dir=. \
		--output-pkg=github.com/replicatedhq/troubleshoot/pkg/client \
		--clientset-name troubleshootclientset \
		--input-base github.com/replicatedhq/troubleshoot/pkg/apis \
		--input troubleshoot/v1beta1 \
		--input troubleshoot/v1beta2 \
		--go-header-file ./hack/boilerplate.go.txt
	cp -r troubleshootclientset pkg/client
	rm -rf troubleshootclientset

.PHONY: openapischema
openapischema: controller-gen
	controller-gen crd +output:dir=./config/crds  paths=./pkg/apis/troubleshoot/v1beta1
	controller-gen crd +output:dir=./config/crds  paths=./pkg/apis/troubleshoot/v1beta2

check-schemas: generate schemas
	@if [ -n "$$(git status --short)" ]; then \
    	echo -e "\033[31mThe git repo is dirty :( Ensure all generated files are committed e.g CRD schema files\033[0;m"; \
    	git status --short; \
    	exit 1; \
	fi

.PHONY: schemas
schemas: openapischema bin/schemagen
	./bin/schemagen --output-dir ./schemas

bin/schemagen:
	go build ${LDFLAGS} -o bin/schemagen github.com/replicatedhq/troubleshoot/cmd/schemagen

.PHONY: docs
docs: fmt vet bin/docsgen
	./bin/docsgen

bin/docsgen:
	go build ${LDFLAGS} -o bin/docsgen github.com/replicatedhq/troubleshoot/cmd/docsgen

controller-gen:
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0
CONTROLLER_GEN=$(shell which controller-gen)

.PHONY: client-gen
client-gen:
	go install k8s.io/code-generator/cmd/client-gen@v0.34.0
CLIENT_GEN=$(shell which client-gen)

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
	curl -sL https://git.io/goreleaser | bash -s -- --rm-dist --snapshot --config deploy/.goreleaser.yaml
	docker tag replicated/troubleshoot:alpha localhost:32000/troubleshoot:alpha
	docker tag replicated/preflight:alpha localhost:32000/preflight:alpha
	docker push localhost:32000/troubleshoot:alpha
	docker push localhost:32000/preflight:alpha

.PHONY: run-preflight
run-preflight: bin/preflight
	./bin/preflight ./examples/preflight/sample-preflight.yaml

.PHONY: run-support-bundle
run-support-bundle: bin/support-bundle
	./bin/support-bundle ./examples/support-bundle/sample-supportbundle.yaml

.PHONY: run-analyze
run-analyze: bin/analyze
	./bin/analyze --analyzers ./examples/support-bundle/sample-analyzers.yaml ./support-bundle.tar.gz

.PHONY: init-sbom
init-sbom:
	mkdir -p sbom/spdx sbom/assets

.PHONY: install-spdx-sbom-generator
install-spdx-sbom-generator: init-sbom
	./scripts/initialize-sbom-build.sh

SPDX_GENERATOR=./sbom/spdx-sbom-generator

.PHONY: generate-sbom
generate-sbom: install-spdx-sbom-generator
	$(SPDX_GENERATOR) -o ./sbom/spdx

sbom/assets/troubleshoot-sbom.tgz: generate-sbom
	tar -czf sbom/assets/troubleshoot-sbom.tgz sbom/spdx/*.spdx

sbom: sbom/assets/troubleshoot-sbom.tgz
	cosign sign-blob \
		--key ./cosign.key \
		--tlog-upload \
		--yes \
		--rekor-url=https://rekor.sigstore.dev \
		sbom/assets/troubleshoot-sbom.tgz > sbom/assets/troubleshoot-sbom.tgz.sig
	cosign public-key --key cosign.key --outfile sbom/assets/key.pub

.PHONY: scan
scan:
	trivy fs \
		--scanners vuln \
		--exit-code=1 \
		--severity="HIGH,CRITICAL" \
		--ignore-unfixed \
		./

.PHONY: watch
watch: npm-install
	bin/watch.js

## Syncronize the code with a remote server. More info: CONTRIBUTING.md
.PHONY: watchrsync
watchrsync: npm-install
	bin/watchrsync.js

.PHONY: npm-install
npm-install:
	npm --version 2>&1 >/dev/null || ( echo "npm not installed; install npm to set up watchrsync" && exit 1 )
	npm list gaze-run-interrupt || npm install install --no-save gaze-run-interrupt@~2.0.0


######## Lagacy make targets ###########
# Deprecated: These can be removed
.PHONY: run-troubleshoot
run-troubleshoot: run-support-bundle

longhorn:
	git clone https://github.com/longhorn/longhorn-manager.git
	cd longhorn-manager && git checkout v1.2.2 && cd ..
	rm -rf pkg/longhorn
	mv longhorn-manager/k8s/pkg pkg/longhorn
	mv longhorn-manager/types pkg/longhorn/types
	mv longhorn-manager/util pkg/longhorn/util
	rm -rf pkg/longhorn/util/daemon
	rm -rf pkg/longhorn/util/server
	find pkg/longhorn -type f | xargs sed -i "s/github.com\/longhorn\/longhorn-manager\/k8s\/pkg/github.com\/replicatedhq\/troubleshoot\/pkg\/longhorn/g"
	find pkg/longhorn -type f | xargs sed -i "s/github.com\/longhorn\/longhorn-manager\/types/github.com\/replicatedhq\/troubleshoot\/pkg\/longhorn\/types/g"
	find pkg/longhorn -type f | xargs sed -i "s/github.com\/longhorn\/longhorn-manager\/util/github.com\/replicatedhq\/troubleshoot\/pkg\/longhorn\/util/g"
	rm -rf longhorn-manager