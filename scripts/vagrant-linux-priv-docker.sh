#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


# Source: https://docs.docker.com/engine/install/ubuntu/

# Add the Docker repository
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
add-apt-repository \
	  "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
	$(lsb_release -cs) \
	stable"

# Update with i386, Go and Docker
apt-get update

apt-get install -y docker-ce docker-ce-cli containerd.io

# Restart Docker in case it got upgraded
systemctl restart docker.service

# Ensure Docker can be used by vagrant user
usermod -aG docker vagrant
