#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -o errexit

VERSION=1.8.4
DOWNLOAD=https://releases.hashicorp.com/vault/${VERSION}/vault_${VERSION}_linux_amd64.zip

function install_vault() {
	if [[ -e /usr/bin/vault ]] ; then
		if [ "v${VERSION}" = "$(vault version | head -n1 | awk '{print $2}')" ] ; then
			return
		fi
	fi
	
	curl -sSL --fail -o /tmp/vault.zip ${DOWNLOAD}

	unzip -d /tmp /tmp/vault.zip
	mv /tmp/vault /usr/bin/vault
	chmod +x /usr/bin/vault
}

install_vault
