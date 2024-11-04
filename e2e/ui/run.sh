#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -eu

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

  ci     For use in CI: runs the proxy, then tests, then stops the proxy.

  Environment Variables:
  NOMAD_ADDR    Address of Nomad cluster or reverse proxy.
  NOMAD_TOKEN   Authentication token.

EOF
}


IMAGE="mcr.microsoft.com/playwright:v1.48.0-noble"
pushd $(dirname "${BASH_SOURCE[0]}") > /dev/null

run_tests() {
    run bash script.sh $@
}

run_shell() {
    run bash $@
}

run() {
    local tty_args=''
    [ -t 1 ] && tty_args='-it'
    docker run $tty_args --rm \
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
  # sending these outputs to stderr so that 'export NOMAD_ADDR=' is the
  # only stdout line, for users to eval, or this script to write then source.
  nomad namespace apply proxy 1>&2
  nomad job run ./input/proxy.nomad 1>&2
  set +e
  IP="$(_get_aws_ip)"
  [ -n "$IP" ] || {
    >&2 echo 'falling back to service IP'
    IP="$(_get_svc_ip)"
  }
  set -e
  [ -n "$IP" ] || {
    >&2 echo 'unable to get an IP for nomad proxy...'
    exit 1 # bad form to exit from a function, but this is essential (and eval'd)
  }
  echo "export NOMAD_ADDR=https://$IP:6464"
}

_get_aws_ip(){
  aws_metadata_url="http://169.254.169.254/latest/meta-data"
  nomad exec -namespace=proxy -job nomad-proxy \
    curl -s "$aws_metadata_url/public-ipv4"
}

_get_svc_ip() {
  nomad service info -namespace=proxy \
    -t '{{ range . }}{{ .Address }}{{ end }}' \
    nomad-proxy
}

stop_proxy() {
  # make sure addr isn't still pointed at the proxy
  export NOMAD_ADDR="${NOMAD_ADDR/6464/4646}"
  nomad job stop -purge -namespace=proxy nomad-proxy
  nomad namespace delete proxy
}

run_ci() {
  set -x
  run_proxy > /tmp/proxy_addr.env
  source /tmp/proxy_addr.env
  run_tests
  rc=$?
  stop_proxy
  exit $rc
}

opt="$1"
case $opt in
    help|--help|-h) help ;;
    proxy|--proxy) run_proxy ;;
    test|--test) shift ; run_tests "$@" ;;
    stop|--stop) stop_proxy ;;
    ci|--ci) run_ci ;;
    shell) shift ; run_shell "$@" ;;
    *) run_tests "$@" ;;
esac
