#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -o errexit

# Minimal effort to support amd64 and arm64 installs.
ARCH=""
case $(arch) in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
esac

VERSION=1.13.0
DOWNLOAD=https://releases.hashicorp.com/vault/${VERSION}/vault_${VERSION}_linux_${ARCH}.zip

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
