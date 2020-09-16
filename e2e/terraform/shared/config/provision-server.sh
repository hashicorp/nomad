#!/bin/bash
# installs and configures the desired build of Nomad as a server
set -o errexit
set -o nounset

# Consul
CONSUL_SRC=/opt/config/full-cluster/consul
CONSUL_DEST=/etc/consul.d

sudo cp "$CONSUL_SRC/server/server.json" "$CONSUL_DEST/"
sudo cp "$CONSUL_SRC/base.json" "$CONSUL_DEST/"
sudo cp "$CONSUL_SRC/aws.json" "$CONSUL_DEST/"

sudo systemctl enable consul.service
sudo systemctl daemon-reload
sudo systemctl restart consul.service
sleep 10

# Vault
VAULT_SRC=/opt/config/full-cluster/vault
VAULT_DEST=/etc/vault.d

sudo cp "$VAULT_SRC/server/vault.hcl" "$VAULT_DEST"

sudo systemctl enable vault.service
sudo systemctl daemon-reload
sudo systemctl restart vault.service

# Nomad

NOMAD_SRC=/opt/config/full-cluster/nomad
NOMAD_DEST=/etc/nomad.d

# assert Nomad binary's permissions
if [[ -f /usr/local/bin/nomad ]]; then
    sudo chmod 0755 /usr/local/bin/nomad
    sudo chown root:root /usr/local/bin/nomad
fi

sudo cp "$NOMAD_SRC/base.hcl" "$NOMAD_DEST/"
sudo cp "$NOMAD_SRC/server/server.hcl" "$NOMAD_DEST/"

sudo systemctl enable nomad.service
sudo systemctl daemon-reload
sudo systemctl restart nomad.service
