#!/usr/bin/env bash

set -euo pipefail

readonly PRJ_ROOT=$(dirname $(dirname -- "$( readlink -f -- $0)"))
tmpdir=""
function cleanup() {
    test -d "$tmpdir" && rm -r "$tmpdir" || :
}

function recreate_tmpdir() {
    cleanup
    tmpdir="$(mktemp -d)"
}
# Cleanup on exit
trap cleanup EXIT

bundle_archive_name="support-bundle.tar.gz"
bundle_directory_name="support-bundle"

echo "====== Generating support bundle from k8s cluster ======"
recreate_tmpdir
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

base_path="$tmpdir/$bundle_directory_name/cluster-resources"
folders=("auth-cani-list" "configmaps" "daemonsets" "endpoints" "events" "deployments" "leases" "services" "pvcs" "pvcs" "jobs" "roles" "statefulsets" "network-policy" "pods" "resource-quota" "rolebindings" "serviceaccounts")

files=("namespaces" "volumeattachments" "pvs" "groups" "nodes" "priorityclasses" "resources")

for folder in "${folders[@]}"; do
    if [ -d "$base_path/$folder" ]; then
        echo "$folder directory was collected"
        if [ "$(ls -A $base_path/$folder)" ]; then
            echo "$folder directory is not empty"
        else
            echo "$folder directory is empty"
            exit 1
        fi
    else
        echo "The $folder folder does not exist in $base_path path."
        exit 1
    fi
done

for file in "${files[@]}"; do
    if [ -e "$base_path/$file.json" ]
    then
        echo "$file.json file was collected"
    else
        echo "The $file.json file does not exist in $base_path path."
        exit 1
    fi
done

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
redacted_archive_name="$tmpdir/redacted-support-bundle.tar.gz"
./bin/support-bundle redact examples/redact/e2e.yaml --bundle=$tmpdir/$bundle_archive_name --output=$redacted_archive_name
if [ $? -ne 0 ]; then
    echo "support-bundle redact command failed"
    exit $?
fi

if ! tar -xvzf $redacted_archive_name --directory $tmpdir; then
    echo "Failed to extract redacted support bundle archive"
    exit 1
fi

if ! grep "\*\*\*HIDDEN\*\*\*" "$tmpdir/$bundle_directory_name/static-hi.log"; then
    echo "$(cat $tmpdir/$bundle_directory_name/static-hi.log)"
    echo "Hidden content not found in redacted static-hi.log file"
    exit 1
fi

echo "======= Generating support bundle from k8s cluster using --load-cluster-specs ======"
recreate_tmpdir
kubectl apply -f "$PRJ_ROOT/testdata/supportbundle/labelled-specs"
./bin/support-bundle -v1 --interactive=false --load-cluster-specs --output=$tmpdir/$bundle_archive_name
if [ $? -ne 0 ]; then
    echo "support-bundle command failed"
    exit $?
fi

if ! tar -xvzf $tmpdir/$bundle_archive_name --directory $tmpdir; then
    echo "A valid support bundle archive was not generated"
    exit 1
fi

if ! grep "labelled-support-bundle-1 \*\*\*HIDDEN\*\*\*" "$tmpdir/$bundle_directory_name/echo-hi-1"; then
    echo "$(cat $tmpdir/$bundle_directory_name/echo-hi-1)"
    echo "Hidden content not found in redacted echo-hi-1 file"
    exit 1
fi

if ! grep "labelled-support-bundle-2 \*\*\*HIDDEN\*\*\*" "$tmpdir/$bundle_directory_name/echo-hi-2"; then
    echo "$(cat $tmpdir/$bundle_directory_name/echo-hi-2)"
    echo "Hidden content not found in redacted echo-hi-2 file"
    exit 1
fi

if ! grep "labelled-support-bundle-3 \*\*\*HIDDEN\*\*\*" "$tmpdir/$bundle_directory_name/echo-hi-3"; then
    echo "$(cat $tmpdir/$bundle_directory_name/echo-hi-3)"
    echo "Hidden content not found in redacted echo-hi-3 file"
    exit 1
fi

if ! grep "labelled-support-bundle-4 \*\*\*HIDDEN\*\*\*" "$tmpdir/$bundle_directory_name/echo-hi-4"; then
    echo "$(cat $tmpdir/$bundle_directory_name/echo-hi-4)"
    echo "Hidden content not found in redacted echo-hi-4 file"
    exit 1
fi
