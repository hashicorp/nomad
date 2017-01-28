#!/bin/bash
set -e

# Configure rkt networking
sudo mkdir -p /etc/rkt/net.d
if [[ -f /etc/rkt/net.d/99-network.conf ]]; then
    echo "rkt network already configured; Skipping"
    exit
fi
echo '{"name": "default", "type": "ptp", "ipMasq": false, "ipam": { "type": "host-local", "subnet": "172.16.28.0/24", "routes": [ { "dst": "0.0.0.0/0" } ] } }' | jq . | sudo tee -a /etc/rkt/net.d/99-network.conf

