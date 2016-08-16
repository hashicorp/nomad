#!/bin/bash

set -ex

VAULT_VERSION="0.6.0"
CURDIR=`pwd`

echo Fetching Vault ${VAULT_VERSION}...
cd /tmp/
wget https://releases.hashicorp.com/vault/${VAULT_VERSION}/vault_${VAULT_VERSION}_linux_amd64.zip -O vault.zip
echo Installing Vault...
unzip vault.zip
sudo chmod +x vault
sudo mv vault /usr/bin/vault
cd ${CURDIR}
