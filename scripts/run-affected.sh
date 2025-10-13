#!/usr/bin/env bash
set -euo pipefail

# 0) Preconditions (one-time)
export PATH="$(go env GOPATH)/bin:$PATH"
go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0 >/dev/null
go install k8s.io/code-generator/cmd/client-gen@v0.34.0 >/dev/null
git fetch origin main --depth=1 || true

# 1) Compute base (robust to unrelated histories)
BASE="$(git merge-base HEAD origin/main 2>/dev/null || true)"
if [ -z "${BASE}" ]; then
  echo "No merge-base with origin/main → running full set"
  PKGS="./..."
  E2E_OUT="$(go run ./scripts/affected-packages.go -mode=suites -changed-files go.mod || true)"
else
  PKGS="$(go run ./scripts/affected-packages.go -base "${BASE}")"
  E2E_OUT="$(go run ./scripts/affected-packages.go -mode=suites -base "${BASE}")"
fi

# 2) Print what will run
echo "=== Affected unit packages ==="
if [ -n "${PKGS}" ]; then echo "${PKGS}"; else echo "(none)"; fi
echo
echo "=== Affected e2e tests ==="
if [ -n "${E2E_OUT}" ]; then echo "${E2E_OUT}"; else echo "(none)"; fi
echo

# 3) Unit tests via Makefile (inherits required build tags)
if [ "${PKGS}" = "./..." ]; then
  echo "Running: make test (all)"
  make test
elif [ -n "${PKGS}" ]; then
  echo "Running: make test-packages for affected pkgs"
  PACKAGES="$(echo "${PKGS}" | xargs)" make test-packages
else
  echo "No affected unit packages"
fi

# 4) E2E tests via Makefile (filtered by regex)
PRE="$(echo "${E2E_OUT}" | awk -F: '$1=="preflight"{print $2}' | paste -sd'|' -)"
SB="$( echo "${E2E_OUT}" | awk -F: '$1=="support-bundle"{print $2}' | paste -sd'|' -)"

if [ -n "${PRE}" ]; then
  echo "Running preflight e2e: ${PRE}"
  RUN="^((${PRE}))$" make support-bundle-e2e-go-test
fi
if [ -n "${SB}" ]; then
  echo "Running support-bundle e2e: ${SB}"
  RUN="^((${SB}))$" make support-bundle-e2e-go-test
fi