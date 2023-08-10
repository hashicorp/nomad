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

VERSION="1.15.1"
DOWNLOAD=https://releases.hashicorp.com/consul/${VERSION}/consul_${VERSION}_linux_${ARCH}.zip

function install_consul() {
	if [[ -e /usr/bin/consul ]] ; then
		if [ "v${VERSION}" == "$(consul version | head -n1 | awk '{print $2}')" ] ; then
			return
		fi
	fi

	curl -sSL --fail -o /tmp/consul.zip ${DOWNLOAD}

	unzip -d /tmp /tmp/consul.zip
	mv /tmp/consul /usr/bin/consul
	chmod +x /usr/bin/consul
}

install_consul
