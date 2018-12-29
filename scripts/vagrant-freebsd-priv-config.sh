#!/bin/sh

chown vagrant:wheel \
       /opt/gopath \
       /opt/gopath/src \
       /opt/gopath/src/github.com \
       /opt/gopath/src/github.com/hashicorp

mkdir -p /usr/local/etc/pkg/repos

cat <<EOT > /usr/local/etc/pkg/repos/FreeBSD.conf
FreeBSD: {
	url: "pkg+http://pkg.FreeBSD.org/\${ABI}/latest"
}
EOT

pkg update

pkg install -y \
       editors/vim-lite \
       devel/git \
       devel/gmake \
       lang/go \
       security/ca_root_nss \
       shells/bash

chsh -s /usr/local/bin/bash vagrant
chsh -s /usr/local/bin/bash root

cat <<EOT >> /home/vagrant/.profile
export GOPATH=/opt/gopath
export PATH=\$GOPATH/bin:\$PATH

cd /opt/gopath/src/github.com/hashicorp/nomad
EOT
