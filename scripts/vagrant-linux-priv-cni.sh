#!/usr/bin/env bash

set -o errexit

ARCH="amd64"
if ( which go > /dev/null ); then
  ARCH=$(go env GOARCH)
fi

VERSION="v1.0.0"
DOWNLOAD=https://github.com/containernetworking/plugins/releases/download/${VERSION}/cni-plugins-linux-${ARCH}-${VERSION}.tgz
TARGET_DIR=/opt/cni/bin
CONFIG_DIR=/opt/cni/config

function install_cni() {
	echo "Installing CNI Plugins..."
	mkdir -p ${TARGET_DIR} ${CONFIG_DIR}
	if [[ -e ${TARGET_DIR}/${VERSION} ]] ; then
		return
	fi

	curl -sSL --fail -o /tmp/cni-plugins.tar.gz ${DOWNLOAD}
	tar -xf /tmp/cni-plugins.tar.gz -C ${TARGET_DIR}
	echo "  - Installed ${VERSION} with $(ls ${TARGET_DIR}|wc -l) plugins."
	touch ${TARGET_DIR}/${VERSION}
}

install_cni
