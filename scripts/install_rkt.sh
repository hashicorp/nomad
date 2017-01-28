#!/bin/bash

set -e

RKT_VERSION="v1.18.0"
CMD="cp"

if [ ! -v DEST_DIR ]; then
	DEST_DIR="/usr/local/bin"
	CMD="sudo cp"
fi

if [[ $(which rkt >/dev/null && rkt version | head -n 1) == "rkt Version: 1.18.0" ]]; then
    echo "rkt installed; Skipping"
else
    printf "Fetching rkt-%s.tar.gz\n" "${RKT_VERSION}"
    cd /tmp
    wget -q https://github.com/coreos/rkt/releases/download/$RKT_VERSION/rkt-$RKT_VERSION.tar.gz -O rkt.tar.gz
    tar xzf rkt.tar.gz

    $CMD rkt-$RKT_VERSION/rkt $DEST_DIR
    $CMD rkt-$RKT_VERSION/*.aci $DEST_DIR
fi

rkt version
