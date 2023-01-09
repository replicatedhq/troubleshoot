
# Image URL to use all building/pushing image targets
IMG ?= controller:latest

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
	-s -w \
	-X ${VERSION_PACKAGE}.version=${VERSION} \
	-X ${VERSION_PACKAGE}.gitSHA=${GIT_SHA} \
	-X ${VERSION_PACKAGE}.buildTime=${DATE} \
"
endef

BUILDFLAGS = -tags "netgo containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp" -installsuffix netgo

all: test support-bundle preflight collect analyze

.PHONY: ffi
ffi: fmt vet
	go build ${BUILDFLAGS} ${LDFLAGS} -o bin/troubleshoot.so -buildmode=c-shared ffi/main.go

.PHONY: test
test: generate fmt vet
	go test ${BUILDFLAGS} ./pkg/... ./cmd/... -coverprofile cover.out

# Go tests that require a K8s instance
# TODOLATER: merge with test, so we get unified coverage reports? it'll add 21~sec to the test job though...
.PHONY: test-integration
test-integration:
	go test --tags integration ./pkg/... ./cmd/...

.PHONY: preflight-e2e-test
preflight-e2e-test:
	./test/validate-preflight-e2e.sh

.PHONY: support-bundle-e2e-test
support-bundle-e2e-test:
	./test/validate-support-bundle-e2e.sh

.PHONY: support-bundle
support-bundle:
	go build ${BUILDFLAGS} ${LDFLAGS} -o bin/support-bundle github.com/replicatedhq/troubleshoot/cmd/troubleshoot

.PHONY: preflight
preflight:
	go build ${BUILDFLAGS} ${LDFLAGS} -o bin/preflight github.com/replicatedhq/troubleshoot/cmd/preflight

.PHONY: analyze
analyze:
	go build ${BUILDFLAGS} ${LDFLAGS} -o bin/analyze github.com/replicatedhq/troubleshoot/cmd/analyze

.PHONY: collect
collect:
	go build ${BUILDFLAGS} ${LDFLAGS} -o bin/collect github.com/replicatedhq/troubleshoot/cmd/collect

.PHONY: fmt
fmt:
	go fmt ./pkg/... ./cmd/...

.PHONY: vet
vet:
	go vet ${BUILDFLAGS} ./pkg/... ./cmd/...

.PHONY: generate
generate: controller-gen client-gen
	$(CONTROLLER_GEN) \
		object:headerFile=./hack/boilerplate.go.txt paths=./pkg/apis/...
	$(CLIENT_GEN) \
		--output-base=./../../../ \
		--output-package=github.com/replicatedhq/troubleshoot/pkg/client \
		--clientset-name troubleshootclientset \
		--input-base github.com/replicatedhq/troubleshoot/pkg/apis \
		--input troubleshoot/v1beta1 \
		--input troubleshoot/v1beta2 \
		-h ./hack/boilerplate.go.txt

.PHONY: openapischema
openapischema: controller-gen
	controller-gen crd +output:dir=./config/crds  paths=./pkg/apis/troubleshoot/v1beta1
	controller-gen crd +output:dir=./config/crds  paths=./pkg/apis/troubleshoot/v1beta2

check-schemas: generate schemas
	@if [ -n "$(shell git status --short)" ]; then \
    	echo -e "\033[31mThe git repo is dirty :( Ensure all generated files are committed e.g CRD schema files\033[0;m"; \
    	git status --short; \
    	exit 1; \
	fi

.PHONY: schemas
schemas: fmt vet openapischema
	go build ${LDFLAGS} -o bin/schemagen github.com/replicatedhq/troubleshoot/cmd/schemagen
	./bin/schemagen --output-dir ./schemas

.PHONY: docs
docs: fmt vet
	go build ${LDFLAGS} -o bin/docsgen github.com/replicatedhq/troubleshoot/cmd/docsgen
	./bin/docsgen

controller-gen:
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0
CONTROLLER_GEN=$(shell which controller-gen)

.PHONY: client-gen
client-gen:
ifeq (, $(shell which client-gen))
	go install k8s.io/code-generator/cmd/client-gen@v0.22.2
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
	curl -sL https://git.io/goreleaser | bash -s -- --rm-dist --snapshot --config deploy/.goreleaser.yaml
	docker tag replicated/troubleshoot:alpha localhost:32000/troubleshoot:alpha
	docker tag replicated/preflight:alpha localhost:32000/preflight:alpha
	docker push localhost:32000/troubleshoot:alpha
	docker push localhost:32000/preflight:alpha

.PHONY: run-preflight
run-preflight: preflight
	./bin/preflight ./examples/preflight/sample-preflight.yaml

.PHONY: run-troubleshoot
run-troubleshoot: support-bundle
	./bin/support-bundle ./examples/support-bundle/sample-supportbundle.yaml

.PHONY: run-analyze
run-analyze: analyze
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
	cosign sign-blob -key cosign.key sbom/assets/troubleshoot-sbom.tgz > sbom/assets/troubleshoot-sbom.tgz.sig
	cosign public-key -key cosign.key -outfile sbom/assets/key.pub

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

.PHONY: scan
scan:
	trivy fs \
		--security-checks vuln \
		--exit-code=1 \
		--severity="HIGH,CRITICAL" \
		--ignore-unfixed \
		./

.PHONY: lint
lint:
	golangci-lint run --new -c .golangci.yaml pkg/... cmd/...

.PHONY: lint-and-fix
lint-and-fix:
	golangci-lint run --new --fix -c .golangci.yaml pkg/... cmd/...
