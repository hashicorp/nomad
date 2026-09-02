#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1


# Install Development utilities
apt-get install -y \
	      default-jre \
	      htop \
	      qemu-system \
	      silversearcher-ag \
	      vim

# Install Chrome for running tests (in headless mode)
if [ "$(dpkg --print-architecture)" = "amd64" ]; then
	mkdir -p /etc/apt/keyrings
	curl -sSL https://dl-ssl.google.com/linux/linux_signing_key.pub \
		| gpg --dearmor -o /etc/apt/keyrings/google-chrome.gpg
	echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/google-chrome.gpg] https://dl.google.com/linux/chrome/deb/ stable main" \
		> /etc/apt/sources.list.d/google-chrome.list
	apt-get update
	apt-get install -y google-chrome-stable
fi

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
