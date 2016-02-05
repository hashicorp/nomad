#!/bin/bash

set -ex

RKT_VERSION="v1.0.0"
DEST_DIR="/usr/local/bin"

wget https://github.com/coreos/rkt/releases/download/$RKT_VERSION/rkt-$RKT_VERSION.tar.gz
tar xzvf rkt-$RKT_VERSION.tar.gz
sudo cp rkt-$RKT_VERSION/rkt $DEST_DIR
sudo cp rkt-$RKT_VERSION/*.aci $DEST_DIR
