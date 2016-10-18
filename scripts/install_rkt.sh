#!/bin/bash

set -ex

RKT_VERSION="v1.17.0"
RKT_SHA512="30fd15716e148afa34ed28e6d5d778226e5e9761e9df3eb98f397cb2a7f3e3fc78e3dad2b717eee4157afc58183778cb1872aa82f3d05cc2bc9fb41193e81a7f"
CMD="cp"

if [ ! -v DEST_DIR ]; then
	DEST_DIR="/usr/local/bin"
	CMD="sudo cp"
fi

if [ ! -d "rkt-${RKT_VERSION}" ]; then
    printf "rkt-%s/ doesn't exist\n" "${RKT_VERSION}"
    if [ ! -f "rkt-${RKT_VERSION}.tar.gz" ]; then
        printf "Fetching rkt-%s.tar.gz\n" "${RKT_VERSION}"
	echo "$RKT_SHA512  rkt-${RKT_VERSION}.tar.gz" > rkt-$RKT_VERSION.tar.gz.sha512sum
        wget https://github.com/coreos/rkt/releases/download/$RKT_VERSION/rkt-$RKT_VERSION.tar.gz
	sha512sum --check rkt-$RKT_VERSION.tar.gz.sha512sum
        tar xzvf rkt-$RKT_VERSION.tar.gz
    fi
fi

$CMD rkt-$RKT_VERSION/rkt $DEST_DIR
$CMD rkt-$RKT_VERSION/*.aci $DEST_DIR

rkt version
