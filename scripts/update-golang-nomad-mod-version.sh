#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

if [ -z "${1:-}" ]; then
    echo "usage: $0 MODULE_VERSION"
    echo "(run in project directory)"
    echo "For example: $0 v2"
    exit 1
fi

mod_version="$1"

# Read current module path from go.mod
current_module=$(grep '^module ' go.mod | awk '{print $2}')
if [ -z "${current_module}" ]; then
    echo "unable to find current module in go.mod"
    exit 1
fi

new_module="github.com/hashicorp/nomad/${mod_version}"
base_module="github.com/hashicorp/nomad"

echo "--> Replacing ${current_module} with ${new_module} ..."

# Update go.mod
go mod edit -module "${new_module}"

# Update Go source files.
#
# The API package has its own go module, so ignore that directory. Do not touch
# the generated proto files, this will be handled by code generation later on
# via the makefile target.
#
# The sed command uses a double-pass, so that changes to Nomad packages that
# import the API package do not include the new version identifier. The API
# module does not get tagged and is not subject to the versioning change.
find . -name '*.go' \
  -not -path './api/*' \
  -not -name '*.pb.go' \
  -exec sed -i '' \
    -e "s|\"${current_module}/|\"${new_module}/|g" \
    -e "s|\"${new_module}/api|\"${base_module}/api|g" \
    {} +

# Update buf.gen.yaml proto module mappings
sed -i '' -e "s|${current_module}/|${new_module}/|g" tools/buf/buf.gen.yaml

# Update embedded Go import path in the raftutil message type generator. If more
# scripts are identified in the future that need updating, they should be added
# here.
sed -i '' \
  -e "s|${current_module}/|${new_module}/|g" \
  helper/raftutil/generate_msgtypes.sh

# The proto files need to be generated in this instance before the structs.
make proto
make generate-structs
