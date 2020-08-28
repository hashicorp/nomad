#!/bin/bash
# installs and configures the desired build of Nomad as a client
set -o errexit
set -o nounset

CLOUD="$1"
NOMAD_CONFIG="$2"

# Consul
CONSUL_SRC=/ops/shared/consul
CONSUL_DEST=/etc/consul.d

sudo cp "$CONSUL_SRC/base.json" "$CONSUL_DEST/"
sudo cp "$CONSUL_SRC/retry_$CLOUD.json" "$CONSUL_DEST/"

sudo systemctl enable consul.service
sudo systemctl daemon-reload
sudo systemctl restart consul.service
sleep 10

# Nomad

NOMAD_SRC=/ops/shared/nomad
NOMAD_DEST=/etc/nomad.d
NOMAD_CONFIG_FILENAME=$(basename "$NOMAD_CONFIG")

# assert Nomad binary's permissions
if [[ -f /usr/local/bin/nomad ]]; then
    sudo chmod 0755 /usr/local/bin/nomad
    sudo chown root:root /usr/local/bin/nomad
fi

sudo cp "$NOMAD_SRC/base.hcl" "$NOMAD_DEST/"
sudo cp "$NOMAD_SRC/$NOMAD_CONFIG" "$NOMAD_DEST/$NOMAD_CONFIG_FILENAME"

# Setup Host Volumes
sudo mkdir -p /tmp/data

sudo systemctl enable nomad.service
sudo systemctl daemon-reload
sudo systemctl start io.podman
sudo systemctl restart nomad.service
