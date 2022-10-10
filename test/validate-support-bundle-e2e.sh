#!/bin/bash

set -euo pipefail

tmpdir="$(mktemp -d)"
bundle_archive_name="support-bundle.tar.gz"
bundle_directory_name="support-bundle"

./bin/support-bundle --interactive=false examples/support-bundle/e2e.yaml --output=$tmpdir/$bundle_archive_name

EXIT_STATUS=0
if ! tar -xvzf $tmpdir/$bundle_archive_name --directory $tmpdir; then
echo "A valid support bundle archive was not generated"
EXIT_STATUS=1
fi

echo "$(cat $tmpdir/$bundle_directory_name/analysis.json)"

if grep -q "No matching files" "$tmpdir/$bundle_directory_name/analysis.json"; then
echo "Some files were not collected"
EXIT_STATUS=1
fi

EXIT_STATUS=0
jq -r '.[].insight.severity' "$tmpdir/$bundle_directory_name/analysis.json" | while read i; do
    if [ $i == "error" ]; then
        EXIT_STATUS=1
        echo "Analyzers with severity of \"error\" found"
    fi

    if [ $i == "warn" ]; then
        EXIT_STATUS=1
        echo "Analyzers with severity of \"warn\" found"
    fi
done

rm -rf "$tmpdir"

exit $EXIT_STATUS
