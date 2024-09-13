#!/usr/bin/env bash

set -euo pipefail

PREFLIGHT_BIN=$(pwd)/bin/preflight

tmpdir="$(mktemp -d)"
trap cleanup SIGHUP SIGINT SIGTERM EXIT
cleanup() {
    rm -rf $tmpdir
}

reset_tmp() {
    rm -rf "$tmpdir"
    tmpdir="$(mktemp -d)"
}

echo -e "\n========= Running preflights from e2e spec and checking results ========="
$PREFLIGHT_BIN --debug --interactive=false --format=json examples/preflight/e2e.yaml > "$tmpdir/result.json"
if [ $? -ne 0 ]; then
    echo "preflight command failed"
    exit $EXIT_STATUS
fi

cat "$tmpdir/result.json"

EXIT_STATUS=0
if grep -q "was not collected" "$tmpdir/result.json"; then
echo "Some files were not collected"
EXIT_STATUS=1
fi

if (( `jq '.pass | length' "$tmpdir/result.json"` < 1 )); then
echo "No passing preflights found"
EXIT_STATUS=1
fi

if (( `jq '.warn | length' "$tmpdir/result.json"` > 0 )); then
echo "Warnings found"
EXIT_STATUS=1
fi

if (( `jq '.fail | length' "$tmpdir/result.json"` > 0 )); then
echo "Failed preflights found"
EXIT_STATUS=1
fi

echo -e "\n========= Running preflights from stdin using e2e spec ========="
cat examples/preflight/e2e.yaml | $PREFLIGHT_BIN --debug --interactive=false --format=json - > "$tmpdir/result.json"
EXIT_STATUS=$?
if [ $EXIT_STATUS -ne 0 ]; then
    echo "preflight command failed"
    exit $EXIT_STATUS
fi

echo -e "\n========= Running preflights and storing bundle in current working directory ========="
E2E_PREFLIGHT=$(pwd)/examples/preflight/e2e.yaml

# We need a clean slate
reset_tmp
pushd $tmpdir >/dev/null
echo $E2E_PREFLIGHT
cat $E2E_PREFLIGHT | $PREFLIGHT_BIN --debug --interactive=false -
EXIT_STATUS=$?
popd >/dev/null

if [ $EXIT_STATUS -ne 0 ]; then
    echo "preflight command failed"
    exit $EXIT_STATUS
fi

if ls $tmpdir/preflightbundle-*.tar.gz; then
    echo "preflight bundle exists"
else
    echo "Failed to find collected preflight bundle"
    exit 1
fi

exit $EXIT_STATUS
