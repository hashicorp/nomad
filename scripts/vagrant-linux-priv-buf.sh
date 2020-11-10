#!/usr/bin/env bash

set -o errexit

# Make sure you grab the latest version
VERSION=0.30.0
DOWNLOAD=https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf-Linux-x86_64.tar.gz

function install() {
    if [[ -e /usr/local/bin/buf ]] ; then
        if [ "${VERSION}" = "$(buf  --version)" ] ; then
            return
        fi
    fi

    # Download
    curl -sSL --fail "$DOWNLOAD" | tar -C /tmp -xvzf - buf/bin

    # all buf files should be world-wide readable
    chmod -R a+r /tmp/buf/bin/*

    # Move buf binaries to /usr/local/bin/
    mv /tmp/buf/bin/* /usr/local/bin/

    # Link
    ln -s /usr/local/bin/buf /usr/bin/buf
    ln -s /usr/local/bin/protoc-gen-buf-check-breaking /usr/bin/protoc-gen-buf-check-breaking
    ln -s /usr/local/bin/protoc-gen-buf-check-lint /usr/bin/protoc-gen-buf-check-lint

    rm -rf /tmp/buf
}

install
