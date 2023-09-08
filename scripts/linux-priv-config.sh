#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


export DEBIAN_FRONTEND=noninteractive

# Update and ensure we have apt-add-repository
apt-get update
apt-get install -y software-properties-common

# Add i386 architecture (for libraries)
dpkg --add-architecture i386

# Update with i386, Go and Docker
apt-get update

# Install Core build utilities for Linux
apt-get install -y \
	build-essential \
	git \
	libc6-dev-i386 \
	libpcre3-dev \
	linux-libc-dev:i386 \
	pkg-config \
	zip \
	curl \
	jq \
	tree \
	unzip \
	wget

# Install ARM build utilities
apt-get install -y \
	binutils-aarch64-linux-gnu \
	binutils-arm-linux-gnueabihf \
	gcc-aarch64-linux-gnu \
	gcc-arm-linux-gnueabihf \
	gcc-multilib-arm-linux-gnueabihf

# Install Windows build utilities
apt-get install -y \
	binutils-mingw-w64 \
	gcc-mingw-w64

# Ensure everything is up to date
apt-get upgrade -y

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
