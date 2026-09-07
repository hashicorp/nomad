#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2026
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
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
	| sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
echo "deb [arch=${ARCH} signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
	$(lsb_release -cs) stable" \
	| sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Update with i386, Go and Docker
sudo apt-get update

sudo apt-get install -y docker-ce docker-ce-cli containerd.io

# Restart Docker in case it got upgraded
sudo systemctl restart docker.service

# Ensure Docker can be used by the correct user
sudo usermod -aG docker ${USER}
