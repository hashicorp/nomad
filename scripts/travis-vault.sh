#!/usr/bin/env bash

set -o errexit

VERSION=0.10.2
OS="linux"
if [[ "$TRAVIS_OS_NAME" == "osx" ]]; then
    OS="darwin"
fi
DOWNLOAD=https://releases.hashicorp.com/vault/${VERSION}/vault_${VERSION}_${OS}_amd64.zip

function install_vault() {
	if [[ -e /usr/bin/vault ]] ; then
		if [ "v${VERSION}" = "$(vault version | head -n1 | awk '{print $2}')" ] ; then
			return
		fi
	fi
	
	curl -sSL --fail -o /tmp/vault.zip ${DOWNLOAD}

	unzip -d /tmp /tmp/vault.zip
	mv /tmp/vault /usr/local/bin/vault
	chmod +x /usr/local/bin/vault
}

install_vault
