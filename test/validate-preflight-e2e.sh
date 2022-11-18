#!/bin/bash

set -euo pipefail

tmpdir="$(mktemp -d)"

./bin/preflight --debug --interactive=false --format=json examples/preflight/e2e.yaml > "$tmpdir/result.json"
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

rm -rf "$tmpdir"

exit $EXIT_STATUS
