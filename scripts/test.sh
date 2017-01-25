#!/usr/bin/env bash
set -e

GOTEST_TAGS="nomad_test"
if [[ $(uname) == "Linux" ]]; then
	if pkg-config --exists lxc; then
		GOTEST_TAGS="$GOTEST_TAGS lxc"
	fi
fi

# Create a temp dir and clean it up on exit
TEMPDIR=`mktemp -d -t nomad-test.XXX`
trap "rm -rf $TEMPDIR" EXIT HUP INT QUIT TERM

# Build the Nomad binary for the API tests
echo "--> Building nomad"
go build -i -tags "$GOTEST_TAGS" -o $TEMPDIR/nomad || exit 1

# Run the tests
echo "--> Running tests"
GOBIN="`which go`"
sudo -E PATH=$TEMPDIR:$PATH  -E GOPATH=$GOPATH   -E NOMAD_TEST_RKT=1 \
    $GOBIN test -tags "$GOTEST_TAGS" ${GOTEST_FLAGS:--cover -timeout=900s} $($GOBIN list ./... | grep -v /vendor/)

