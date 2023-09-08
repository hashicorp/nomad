#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


# Source: https://docs.docker.com/engine/install/ubuntu/

# Minimal effort to support amd64 and arm64 installs.
ARCH=""
case $(arch) in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
esac

USER=""
case $(whoami) in
    root) USER="vagrant" ;;
    *) USER=$(whoami) ;;
esac

# Add the Docker repository
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo add-apt-repository -y \
	  "deb [arch=${ARCH}] https://download.docker.com/linux/ubuntu \
	$(lsb_release -cs) \
	stable"

# Update with i386, Go and Docker
sudo apt-get update

sudo apt-get install -y docker-ce docker-ce-cli containerd.io

# Restart Docker in case it got upgraded
sudo systemctl restart docker.service

# Ensure Docker can be used by the correct user
sudo usermod -aG docker ${USER}
