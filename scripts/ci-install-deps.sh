#!/bin/bash

set -e

# $1 == goos, $2 == goarch
echo "Installing dependencies for goos:$1 goarch:$2"

#### Install buf CLI ####

VERSION=0.36.0
DOWNLOAD=https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf

if [[ "$1" == "darwin" ]]; then
  DOWNLOAD="${DOWNLOAD}-Darwin-x86_64.tar.gz"
  wget "${DOWNLOAD}" -O - | tar -xz -C /tmp
  mv /tmp/buf/bin/buf /usr/local/bin
  chmod +x /usr/local/bin/buf
  # Exit script with success code; nothing more to do for darwin builds
  exit 0
else
  DOWNLOAD="${DOWNLOAD}-Linux-x86_64.tar.gz"
  wget "${DOWNLOAD}" -O - | tar -xz -C /tmp
  mv /tmp/buf/bin/buf /usr/local/bin
  chmod +x /usr/local/bin/buf
fi

### Install required libraries ####

export DEBIAN_FRONTEND=noninteractive

# Update and ensure we have apt-add-repository
apt-get update
apt-get install -y software-properties-common

# Add i386 architecture (for libraries)
dpkg --add-architecture i386

# Update with i386, Go and Docker
apt-get update

# Install GCC-5
if [[ "$2" != "386" ]]; then
  echo "deb http://dk.archive.ubuntu.com/ubuntu/ xenial main" | sudo tee -a /etc/apt/sources.list >/dev/null
  echo "deb http://dk.archive.ubuntu.com/ubuntu/ xenial universe" | sudo tee -a /etc/apt/sources.list >/dev/null
  sudo apt-get update
  sudo apt-get install g++-5 gcc-5 
  sudo update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-5 5
  sudo update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-5 5
fi

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

# Install 32 bit headers and libraries for linux/386 builds
if [[ "$1" == "linux" && "$2" == "386" ]]; then
    echo "Installing linux/386 dependencies"
    apt-get install gcc-multilib 
fi

# Install ARM build utilities for arm builds
if [[ "$2" == "arm" || "$2" == "arm64" ]]; then
    echo "Installing arm/arm64 dependencies"
    apt-get install -y \
      gcc-5-aarch64-linux-gnu \
      gcc-5-arm-linux-gnueabihf \
      gcc-5-multilib-arm-linux-gnueabihf \
      binutils-aarch64-linux-gnu \
      binutils-arm-linux-gnueabihf 
fi

# Install Windows build utilities for windows builds
if [[ "$1" == "windows" ]]; then
    echo "Installing windows dependencies"
    apt-get install -y \
        binutils-mingw-w64 \
        gcc-mingw-w64
fi
