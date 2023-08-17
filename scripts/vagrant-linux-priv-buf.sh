#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -o errexit

# Make sure you grab the latest version
VERSION=0.36.0
DOWNLOAD=https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf-Linux-x86_64

function install() {
  if command -v buf >/dev/null; then
    if [ "${VERSION}" = "$(buf  --version)" ] ; then
      return
    fi
  fi

  # Download
  curl -sSL --fail "$DOWNLOAD" -o /tmp/buf

  # make executable
  chmod +x /tmp/buf

  # Move buf to /usr/bin
  mv /tmp/buf /usr/bin/buf
}

install
