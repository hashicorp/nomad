#!/usr/bin/env bash

set -o errexit

# Make sure you grab the latest version
VERSION=3.6.1
DOWNLOAD=https://github.com/google/protobuf/releases/download/v${VERSION}/protoc-${VERSION}-linux-x86_64.zip

function install_protoc() {
    if [[ -e /usr/local/bin/protoc ]] ; then
        if [ "${VERSION}" = "$(protoc  --version | cut -d ' ' -f 2)" ] ; then
            return
        fi
    fi

    # Download
    wget -q -O /tmp/protoc.zip ${DOWNLOAD}

    # Unzip
    unzip /tmp/protoc.zip -d /tmp/protoc3

    # all protoc files should be world-wide readable, specially the include files
    chmod -R a+r /tmp/protoc3

    # Move protoc to /usr/local/bin/
    mv /tmp/protoc3/bin/* /usr/local/bin/

    # Move protoc3/include to /usr/local/include/
    mv /tmp/protoc3/include/* /usr/local/include/

    # Link
    ln -s /usr/local/bin/protoc /usr/bin/protoc

    rm -rf /tmp/protoc3 /tmp/protoc.zip
}

install_protoc
