#!/usr/bin/env bash

set -euo pipefail

tmpdir="$(mktemp -d)"
bundle_archive_name="support-bundle.tar.gz"
bundle_directory_name="support-bundle"

echo "====== Generating support bundle from k8s cluster ======"
./bin/support-bundle --debug --interactive=false examples/support-bundle/e2e.yaml --output=$tmpdir/$bundle_archive_name
if [ $? -ne 0 ]; then
    echo "support-bundle command failed"
    exit $?
fi

if ! tar -xvzf $tmpdir/$bundle_archive_name --directory $tmpdir; then
    echo "A valid support bundle archive was not generated"
    exit 1
fi

echo "$(cat $tmpdir/$bundle_directory_name/analysis.json)"

if grep -q "No matching files" "$tmpdir/$bundle_directory_name/analysis.json"; then
    echo "Some files were not collected"
    exit 1
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
if [ $EXIT_STATUS -ne 0 ]; then
    echo "support-bundle command failed"
    exit $EXIT_STATUS
fi

echo "======= Redact an existing support bundle ======"
redact_tmpdir="$(mktemp -d)"
redacted_archive_name="$redact_tmpdir/redacted-support-bundle.tar.gz"
./bin/support-bundle redact examples/redact/e2e.yaml --bundle=$tmpdir/$bundle_archive_name --output=$redacted_archive_name
if [ $? -ne 0 ]; then
    echo "support-bundle redact command failed"
    exit $?
fi

if ! tar -xvzf $redacted_archive_name --directory $redact_tmpdir; then
    echo "Failed to extract redacted support bundle archive"
    exit 1
fi

if ! grep "\*\*\*HIDDEN\*\*\*" "$redact_tmpdir/$bundle_directory_name/static-hi/static-hi.log"; then
    echo "$(cat $redact_tmpdir/$bundle_directory_name/static-hi.log)"
    echo "Hidden content not found in redacted static-hi.log file"
    exit 1
fi

rm -rf "$tmpdir" "$redact_tmpdir"
