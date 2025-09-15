
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

build: tidy
	@echo "Build cli binaries"
	$(MAKE) bin/support-bundle bin/preflight

.PHONY: clean
clean:
	@rm -f bin/support-bundle
	@rm -f bin/preflight

.PHONY: tidy
tidy:
	go mod tidy

# Only build when any of the files in SOURCES changes, or if bin/<file> is absent
MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
SOURCES := $(shell find $(MAKEFILE_DIR) -type f \( -name "*.go" -o -name "go.mod" -o -name "go.sum" \))
bin/support-bundle: $(SOURCES)
	go build ${BUILDFLAGS} ${LDFLAGS} -o bin/support-bundle github.com/replicatedhq/troubleshoot/cmd/troubleshoot

bin/preflight: $(SOURCES)
	go build ${BUILDFLAGS} ${LDFLAGS} -o bin/preflight github.com/replicatedhq/troubleshoot/cmd/preflight

.PHONY: support-bundle
support-bundle: bin/support-bundle

.PHONY: preflight
preflight: bin/preflight

.PHONY: fmt
fmt:
	go fmt ${BUILDPATHS}

.PHONY: vet
vet:
	go vet ${BUILDFLAGS} ${BUILDPATHS}

.PHONY: release
release: export GITHUB_TOKEN = $(shell echo ${GITHUB_TOKEN_TROUBLESHOOT})
release:
	curl -sL https://git.io/goreleaser | bash -s -- --rm-dist --config deploy/.goreleaser.yml

.PHONY: snapshot-release
snapshot-release:
	curl -sL https://git.io/goreleaser | bash -s -- --rm-dist --snapshot --config deploy/.goreleaser.snapshot.yml
	docker push replicated/troubleshoot:alpha
	docker push replicated/preflight:alpha
