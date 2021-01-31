#!/usr/bin/env bash

set -o errexit

VERSION="v0.8.6"
DOWNLOAD=https://github.com/containernetworking/plugins/releases/download/${VERSION}/cni-plugins-linux-amd64-${VERSION}.tgz
TARGET_DIR=/opt/cni/bin

function install_cni() {
	mkdir -p ${TARGET_DIR}
	if [[ -e ${TARGET_DIR}/${VERSION} ]] ; then
		return
	fi

	curl -sSL --fail -o /tmp/cni-plugins.tar.gz ${DOWNLOAD}
	tar -xf /tmp/cni-plugins.tar.gz -C ${TARGET_DIR}
	touch ${TARGET_DIR}/${VERSION}
}

install_cni
