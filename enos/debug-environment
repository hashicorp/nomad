#!/usr/bin/env bash

set -eu

help() {
    cat <<'EOF'
Export a set of environment variables so you can debug Nomad while running it
under this Enos scenario.

Usage: $(debug-environment .enos/[directory with Enos state])

EOF
}

DIR=${1:-unknown}
if [[ $DIR == "unknown" ]]; then
   help
   exit 1
fi

pushd $DIR > /dev/null
cat <<EOF
export NOMAD_TOKEN=$(terraform output --raw nomad_token)
export NOMAD_ADDR=$(terraform output --raw nomad_addr)
export NOMAD_CACERT=$(terraform output --raw ca_file)
export NOMAD_CLIENT_CERT=$(terraform output --raw cert_file)
export NOMAD_CLIENT_KEY=$(terraform output --raw key_file)
EOF
