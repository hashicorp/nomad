#!/bin/bash

set -ex

CONSUL_VERSION="0.7.0"
CURDIR=`pwd`

echo Fetching Consul...
cd /tmp/
wget https://releases.hashicorp.com/consul/${CONSUL_VERSION}/consul_${CONSUL_VERSION}_linux_amd64.zip -O consul.zip
echo Installing Consul...
unzip consul.zip
sudo chmod +x consul
sudo mv consul /usr/bin/consul
cd ${CURDIR}
