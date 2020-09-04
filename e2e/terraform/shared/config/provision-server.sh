#!/bin/bash
# installs and configures the desired build of Nomad as a server
set -o errexit
set -o nounset

CLOUD="$1"
SERVER_COUNT="$2"
NOMAD_CONFIG="$3"

# Consul
CONSUL_SRC=/ops/shared/consul
CONSUL_DEST=/etc/consul.d

sed "s/SERVER_COUNT/$SERVER_COUNT/g" "$CONSUL_SRC/server.json" > /tmp/server.json
sudo mv /tmp/server.json "$CONSUL_DEST/server.json"
sudo cp "$CONSUL_SRC/base.json" "$CONSUL_DEST/"
sudo cp "$CONSUL_SRC/retry_$CLOUD.json" "$CONSUL_DEST/"

sudo systemctl enable consul.service
sudo systemctl daemon-reload
sudo systemctl restart consul.service
sleep 10

# Vault
VAULT_SRC=/ops/shared/vault
VAULT_DEST=/etc/vault.d

sudo cp "$VAULT_SRC/vault.hcl" "$VAULT_DEST"

sudo systemctl enable vault.service
sudo systemctl daemon-reload
sudo systemctl restart vault.service

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

sed "s/3 # SERVER_COUNT/$SERVER_COUNT/g" "$NOMAD_SRC/$NOMAD_CONFIG" \
    > "/tmp/$NOMAD_CONFIG_FILENAME"
sudo mv "/tmp/$NOMAD_CONFIG_FILENAME" "$NOMAD_DEST/$NOMAD_CONFIG_FILENAME"

sudo systemctl enable nomad.service
sudo systemctl daemon-reload
sudo systemctl restart nomad.service
