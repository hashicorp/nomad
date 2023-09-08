#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


# Install Development utilities
apt-get install -y \
	      default-jre \
	      htop \
	      qemu \
	      silversearcher-ag \
	      vim

# Install Chrome for running tests (in headless mode)
curl -sSL -o- https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add -
echo "deb https://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google.list
apt-get update
apt-get install -y google-chrome-stable

# Set hostname -> IP to make advertisement work as expected
ip=$(ip route get 1 | awk '{print $NF; exit}')
hostname=$(hostname)
sed -i -e "s/.*nomad.*/${ip} ${hostname}/" /etc/hosts

# Ensure we cd into the working directory on login
if [ -d /home/vagrant/ ] ; then
  if ! grep "cd /opt/gopath/src/github.com/hashicorp/nomad" /home/vagrant/.profile ; then
    echo 'cd /opt/gopath/src/github.com/hashicorp/nomad' >> /home/vagrant/.profile
  fi
fi
