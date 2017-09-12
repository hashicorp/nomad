#!/usr/bin/env bash

export DEBIAN_FRONTEND=noninteractive

# Update and ensure we have apt-add-repository
sudo apt-get update
sudo apt-get install -y software-properties-common

# Add i386 architecture (for libraries)
sudo dpkg --add-architecture i386

# Add the Docker repository
sudo apt-key adv \
	--keyserver hkp://p80.pool.sks-keyservers.net:80 \
	--recv-keys 9DC858229FC7DD38854AE2D88D81803C0EBFCD88
sudo add-apt-repository \
	"deb [arch=amd64] https://download.docker.com/linux/ubuntu \
	$(lsb_release -cs) \
	stable"

# Update with i386, Go and Docker
sudo apt-get update

# Install Core build utilities for Linux
sudo apt-get install -y \
	build-essential \
	git \
	libc6-dev-i386 \
	liblxc1 \
	libpcre3-dev \
	linux-libc-dev:i386 \
	lxc-dev \
	lxc-templates \
	pkg-config \
	zip

# Install Development utilities
sudo apt-get install -y \
	curl \
	default-jre \
	docker-ce \
	htop \
	jq \
	qemu \
	silversearcher-ag \
	tree \
	unzip \
	vim

# Install ARM build utilities
sudo apt-get install -y \
	binutils-aarch64-linux-gnu \
	binutils-arm-linux-gnueabihf \
	gcc-5-aarch64-linux-gnu \
	gcc-5-arm-linux-gnueabihf \
	gcc-5-multilib-arm-linux-gnueabihf

# Install Windows build utilities
sudo apt-get install -y \
	binutils-mingw-w64 \
	gcc-mingw-w64

# Ensure everything is up to date
sudo apt-get upgrade -y

# Restart Docker in case it got upgraded
sudo systemctl restart docker.service

# Ensure Docker can be used by vagrant user
sudo usermod -aG docker vagrant

# Ensure we cd into the working directory on login
if ! grep "cd /opt/gopath/src/github.com/hashicorp/nomad" /home/vagrant/.profile ; then
	echo 'cd /opt/gopath/src/github.com/hashicorp/nomad' >> /home/vagrant/.profile
fi
