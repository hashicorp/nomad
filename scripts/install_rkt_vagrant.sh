#!/bin/bash

set -ex

# Configure rkt networking
sudo mkdir -p /etc/rkt/net.d
echo '{"name": "default", "type": "ptp", "ipMasq": false, "ipam": { "type": "host-local", "subnet": "172.16.28.0/24", "routes": [ { "dst": "0.0.0.0/0" } ] } }' | sudo tee -a /etc/rkt/net.d/99-network.conf

