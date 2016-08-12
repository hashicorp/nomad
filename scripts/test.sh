#!/usr/bin/env bash

# Create a temp dir and clean it up on exit
TEMPDIR=`mktemp -d -t nomad-test.XXX`
trap "rm -rf $TEMPDIR" EXIT HUP INT QUIT TERM

# Build the Nomad binary for the API tests
echo "--> Building nomad"
go build -o $TEMPDIR/nomad || exit 1

# Run the tests
echo "--> Running tests"
GOBIN="`which go`"
go list ./... | grep -v '^github.com/hashicorp/nomad/vendor/' | \
    sudo \
        -E PATH=$TEMPDIR:$PATH \
        -E GOPATH=$GOPATH \
        xargs $GOBIN test -v ${GOTEST_FLAGS:--cover -timeout=900s}
