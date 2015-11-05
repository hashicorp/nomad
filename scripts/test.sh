#!/usr/bin/env bash

# Create a temp dir and clean it up on exit
TEMPDIR=`mktemp -d -t nomad-test.XXX`
trap "rm -rf $TEMPDIR" EXIT HUP INT QUIT TERM

# Build the Nomad binary for the API tests
echo "--> Building nomad"
go build -o $TEMPDIR/nomad || exit 1

# Run the tests
echo "--> Running tests"
go list ./... | PATH=$TEMPDIR:$PATH xargs -n1 go test -v -cover -timeout=40s
