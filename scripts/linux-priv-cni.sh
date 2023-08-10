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

VERSION="v1.0.0"
DOWNLOAD=https://github.com/containernetworking/plugins/releases/download/${VERSION}/cni-plugins-linux-${ARCH}-${VERSION}.tgz
TARGET_DIR=/opt/cni/bin
CONFIG_DIR=/opt/cni/config

function install_cni() {
	mkdir -p ${TARGET_DIR} ${CONFIG_DIR}
	if [[ -e ${TARGET_DIR}/${VERSION} ]] ; then
		return
	fi

	curl -sSL --fail -o /tmp/cni-plugins.tar.gz ${DOWNLOAD}
	tar -xf /tmp/cni-plugins.tar.gz -C ${TARGET_DIR}
	touch ${TARGET_DIR}/${VERSION}
}

install_cni
