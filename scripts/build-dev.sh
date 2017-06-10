#!/usr/bin/env bash
set -e

GIT_COMMIT="$(git rev-parse HEAD)"
GIT_DIRTY="$(test -n "`git status --porcelain`" && echo "+CHANGES" || true)"
LDFLAG="main.GitCommit=${GIT_COMMIT}${GIT_DIRTY}"

TAGS="nomad_test"
if [[ $(uname) == "Linux" ]]; then
	if pkg-config --exists lxc; then
		TAGS="$TAGS lxc"
	fi
fi

while :; do
    case $1 in
        -ui)
            TAGS="ui $TAGS"
            break
        *)
            echo "usage: build-dev.sh [-ui]"
            exit
    esac

    shift
done

echo "--> Installing with tags: $TAGS"
go install -ldflags "-X $LDFLAG" -tags "$TAGS"

echo "--> Ensuring bin directory exists..."
mkdir -p bin

echo "--> Copying to bin"
cp $GOPATH/bin/nomad bin/nomad
