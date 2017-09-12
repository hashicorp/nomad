#!/usr/bin/env bash

set -o errexit

function install_vault() {
    local version=0.8.2
    local download=https://releases.hashicorp.com/vault/${version}/vault_${version}_linux_amd64.zip

	if [[ -e /usr/bin/vault ]] ; then
		if [ "v${version}" = "$(vault version | head -n1 | awk '{print $2}')" ] ; then
            echo "Vault ${version} already installed"
			return
		fi
	fi
	
	wget -q -O /tmp/vault.zip ${download}

	unzip -d /tmp /tmp/vault.zip
	sudo install /tmp/vault /usr/bin/vault
}

install_vault
