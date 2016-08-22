#!/usr/bin/env bash

# Create a temp dir and clean it up on exit
TEMPDIR=`mktemp -d -t nomad-test.XXX`
trap "rm -rf $TEMPDIR" EXIT HUP INT QUIT TERM

# Build the Nomad binary for the API tests
echo "--> Building nomad"
go build -tags "nomad_test" -o $TEMPDIR/nomad || exit 1

# Run the tests
echo "--> Running tests"
GOBIN="`which go`"
sudo -E PATH=$TEMPDIR:$PATH  -E GOPATH=$GOPATH \
    $GOBIN test -tags "nomad_test" -v ${GOTEST_FLAGS:--cover -timeout=900s} $($GOBIN list ./... | grep -v /vendor/)

