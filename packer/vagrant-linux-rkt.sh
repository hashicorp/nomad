#!/usr/bin/env bash

set -o errexit

VERSION=1.18.0
DOWNLOAD=https://github.com/coreos/rkt/releases/download/v${VERSION}/rkt-v${VERSION}.tar.gz

function install_rkt() {
	if [[ -e /usr/local/bin/rkt ]] ; then
		if [ "rkt Version: ${VERSION}" == "$(rkt version | head -n1)" ] ; then
			return
		fi
	fi
	
	wget -q -O /tmp/rkt.tar.gz "${DOWNLOAD}"

	sudo tar -C /tmp -xvf /tmp/rkt.tar.gz
	sudo mv /tmp/rkt-v${VERSION}/rkt /usr/local/bin
	sudo mv /tmp/rkt-v${VERSION}/*.aci /usr/local/bin
}

function configure_rkt_networking() {
	if [[ -e /etc/rkt/net.d/99-network.conf ]] ; then
		return
	fi

	sudo mkdir -p /etc/rkt/net.d
	sudo tee /etc/rkt/net.d/99-network.conf <<EOT
{
  "name": "default",
  "type": "ptp",
  "ipMasq": false,
  "ipam": {
    "type": "host-local",
    "subnet": "172.16.28.0/24",
    "routes": [
      {
        "dst": "0.0.0.0/0"
      }
    ]
  }
}
EOT
}

install_rkt
configure_rkt_networking
