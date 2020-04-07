#!/usr/bin/env bash

## Script for enabling cross compiling Nomad to OSX using
## cross compiling compiler and https://github.com/tpoechtrager/osxcross . 
##
## Heavily inspired by https://github.com/docker/golang-cross/blob/f5f1a330c7d51531c99bda3319c37b170afe3946/Dockerfile

set -e

# OS-X SDK parameters
# TODO Go 1.13 supports macOS 10.11 (el capitan) and higher. Update this to 10.11 SDK
# once an updated version of the SDK is available on s3.dockerproject.org
OSX_SDK=MacOSX10.10.sdk
OSX_SDK_SUM=631b4144c6bf75bf7a4d480d685a9b5bda10ee8d03dbf0db829391e2ef858789

# OSX-cross parameters. Go 1.11 requires OSX >= 10.10
# TODO Go 1.13 supports macOS 10.11 (el capitan) and higher. Update this to 10.11
# once an updated version of the SDK is available on s3.dockerproject.org
OSX_VERSION_MIN=10.10
OSX_CROSS_COMMIT=a9317c18a3a457ca0a657f08cc4d0d43c6cf8953

OSX_CROSS_PATH=${1:-${HOME}/osxcross}

download_sdk() {
    echo "----> Downloading OSX SDK ${OSX_SDK}"
    mkdir -p "${OSX_CROSS_PATH}/tarballs"
    curl -sSL --fail -o "${OSX_CROSS_PATH}/tarballs/${OSX_SDK}.tar.xz" https://s3.dockerproject.org/darwin/v2/${OSX_SDK}.tar.xz
    echo "${OSX_SDK_SUM}"  "${OSX_CROSS_PATH}/tarballs/${OSX_SDK}.tar.xz" | sha256sum -c -
}

osxcross_deps() {
    echo "----> Installing osx cross dependencies"
    sudo apt-get update -qq || true
    sudo apt-get install -y -q --no-install-recommends \
         clang file llvm patch xz-utils
}

osxcross_fetch() {
    echo "----> fetching osx cross toolchain"
    git clone https://github.com/tpoechtrager/osxcross.git .
    git checkout -q "${OSX_CROSS_COMMIT}"
}

osxcross_build() {
    echo "----> building osx cross toolchain"
    UNATTENDED=yes OSX_VERSION_MIN=${OSX_VERSION_MIN} ./build.sh
}

rm -rf "${OSX_CROSS_PATH}"
mkdir -p "${OSX_CROSS_PATH}"
cd "${OSX_CROSS_PATH}"
osxcross_deps
osxcross_fetch
download_sdk
osxcross_build
