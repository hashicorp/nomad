#!/bin/bash

set -e

RKT_VERSION="v1.18.0"
CMD="cp"

if [ ! -v DEST_DIR ]; then
	DEST_DIR="/usr/local/bin"
	CMD="sudo cp"
fi

if [ ! -d "rkt-${RKT_VERSION}" ]; then
    printf "rkt-%s/ doesn't exist\n" "${RKT_VERSION}"
    if [ ! -f "rkt-${RKT_VERSION}.tar.gz" ]; then
        printf "Fetching rkt-%s.tar.gz\n" "${RKT_VERSION}"
        wget -q https://github.com/coreos/rkt/releases/download/$RKT_VERSION/rkt-$RKT_VERSION.tar.gz
        tar xzvf rkt-$RKT_VERSION.tar.gz
    fi
fi

$CMD rkt-$RKT_VERSION/rkt $DEST_DIR
$CMD rkt-$RKT_VERSION/*.aci $DEST_DIR

rkt version
