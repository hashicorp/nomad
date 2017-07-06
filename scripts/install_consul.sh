#!/bin/bash

set -e

CONSUL_VERSION="0.8.5"
CURDIR=`pwd`

if [[ $(which consul >/dev/null && consul version | head -n 1 | cut -d ' ' -f 2) == "v$CONSUL_VERSION" ]]; then
    echo "Consul v$CONSUL_VERSION already installed; Skipping"
    exit
fi

echo Fetching Consul...
cd /tmp/
wget -q https://releases.hashicorp.com/consul/${CONSUL_VERSION}/consul_${CONSUL_VERSION}_linux_amd64.zip -O consul.zip
echo Installing Consul...
unzip consul.zip
sudo chmod +x consul
sudo mv consul /usr/bin/consul
cd ${CURDIR}
