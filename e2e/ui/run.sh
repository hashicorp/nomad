#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -e

help() {
    cat <<EOF
Usage: run.sh [subcommand] [options] [--help]

  Runs playwright tests in a Docker container against your shell's configured
  Nomad target.

  Subcommands:
  test   Run the tests (default behavior if no subcommand is provided). Options:
         --no-install   Don't run npm install because you've already done so.

  shell  Run a bash shell with the environment already set up. Maybe useful
         for debugging.

  proxy  Deploy a reverse proxy. When the cluster is using mTLS, you will need
         this so that we don't need to load a CA certificate into the browser.
         This reverse proxy uses a self-signed cert. Will print a new NOMAD_ADDR
         address for you to use for test runs.

  Environment Variables:
  NOMAD_ADDR    Address of Nomad cluster or reverse proxy.
  NOMAD_TOKEN   Authentication token.

EOF
}


IMAGE="mcr.microsoft.com/playwright:focal"
pushd $(dirname "${BASH_SOURCE[0]}") > /dev/null

run_tests() {
    run bash script.sh $@
}

run_shell() {
    run bash $@
}

run() {
    exec docker run -it --rm \
           -v $(pwd):/src \
           -w /src \
           -e NOMAD_ADDR=$NOMAD_ADDR \
           -e NOMAD_TOKEN=$NOMAD_TOKEN \
           --ipc=host \
           --net=host \
           "$IMAGE" \
           $@
}

run_proxy() {
    nomad namespace apply proxy
    nomad job run "./input/proxy.nomad"
    IP=$(nomad node status -json -verbose \
          $(nomad operator api '/v1/allocations?namespace=proxy' | jq -r '.[] | select(.JobID == "nomad-proxy") | .NodeID') \
        | jq -r '.Attributes."unique.platform.aws.public-ipv4"')
    echo "NOMAD_ADDR=https://$IP:6464"
    exit 0
}

opt="$1"
case $opt in
    help|--help|-h) help ;;
    proxy|--proxy) run_proxy ;;
    test|--test) shift ; run_tests "$@" ;;
    shell) shift ; run_shell ;;
    *) run_tests "$@" ;;
esac

run_tests
