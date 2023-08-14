#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -o errexit

# Identify the user we are running as. If it's as root, we assume Vagrant
# which isn't great, but is better than the old behaviour.
USER=""
case $(whoami) in
    root) USER="vagrant" ;;
    *) USER=$(whoami) ;;
esac

# Minimal effort to support amd64 and arm64 installs.
ARCH=""
case $(arch) in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
esac

function install_go() {
	local go_version="1.21.0"
	local download="https://storage.googleapis.com/golang/go${go_version}.linux-${ARCH}.tar.gz"

	if go version 2>&1 | grep -q "${go_version}"; then
		return
	fi

	# remove previous older version
	sudo rm -rf /usr/local/go

	if [ -f /tmp/go.tar.gz ] ; then
	  sudo rm -f /tmp/go.tar.gz
	fi

	# retry downloading on spurious failure
	curl -sSL --fail -o /tmp/go.tar.gz \
		--retry 5 --retry-connrefused \
		"${download}"

	tar -C /tmp -xf /tmp/go.tar.gz
	sudo mv /tmp/go /usr/local
	sudo chown -R root:root /usr/local/go
}

install_go

# Ensure that the GOPATH tree is owned by the correct user.
sudo mkdir -p /opt/gopath
sudo chown -R $USER:$USER /opt/gopath

# Ensure Go is on PATH
if [ ! -e /usr/bin/go ] ; then
	sudo ln -s /usr/local/go/bin/go /usr/bin/go
fi
if [ ! -e /usr/bin/gofmt ] ; then
	sudo ln -s /usr/local/go/bin/gofmt /usr/bin/gofmt
fi


# Ensure new sessions know about GOPATH
if sudo test ! -f /etc/profile.d/gopath.sh  ; then
	sudo bash -c 'cat <<EOT > /etc/profile.d/gopath.sh
export GOPATH="/opt/gopath"
export PATH="/opt/gopath/bin:\$PATH"
EOT'
	sudo chmod 755 /etc/profile.d/gopath.sh
fi
