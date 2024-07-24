#!/usr/bin/env bash
set -eu # exit in error, exit if vars not set

# TODO: When we add more examples, we should add some logic here to find all
#       directories with a go.mod file and run the main.go application there

# function run() {
#     local EXAMPLE_PATH=$1
#     pushd $EXAMPLE_PATH > /dev/null
#     echo "Running \"$EXAMPLE_PATH\" example"
#     go mod tidy && go run main.go
#     popd > /dev/null
# }

# run examples/sdk/helm-template/

bin/preflight --interactive=false examples/preflight/host/evans.yaml
