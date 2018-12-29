#!/usr/bin/env bash

set -o errexit

VERSION=0.7.0
DOWNLOAD=https://releases.hashicorp.com/vault/${VERSION}/vault_${VERSION}_linux_amd64.zip

function install_vault() {
	if [[ -e /usr/bin/vault ]] ; then
		if [ "v${VERSION}" = "$(vault version | head -n1 | awk '{print $2}')" ] ; then
			return
		fi
	fi
	
	wget -q -O /tmp/vault.zip ${DOWNLOAD}

	unzip -d /tmp /tmp/vault.zip
	mv /tmp/vault /usr/bin/vault
	chmod +x /usr/bin/vault
}

install_vault
