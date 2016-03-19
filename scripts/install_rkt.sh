#!/bin/bash

set -ex

RKT_VERSION="v1.2.0"
DEST_DIR="/usr/local/bin"

sudo mkdir -p /etc/rkt/net.d
echo '{"name": "default", "type": "ptp", "ipMasq": false, "ipam": { "type": "host-local", "subnet": "172.16.28.0/24", "routes": [ { "dst": "0.0.0.0/0" } ] } }' | sudo tee -a /etc/rkt/net.d/99-network.conf

wget https://github.com/coreos/rkt/releases/download/$RKT_VERSION/rkt-$RKT_VERSION.tar.gz
tar xzvf rkt-$RKT_VERSION.tar.gz
sudo cp rkt-$RKT_VERSION/rkt $DEST_DIR
sudo cp rkt-$RKT_VERSION/*.aci $DEST_DIR

rkt version
