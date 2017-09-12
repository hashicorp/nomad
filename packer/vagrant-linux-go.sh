#!/usr/bin/env bash

function install_go() {
	local go_version=1.9
	local download=
	
	download="https://storage.googleapis.com/golang/go${go_version}.linux-amd64.tar.gz"

    if [ -x /usr/local/go/bin/go ] ; then
        if [ "$(go version)" = "go version go${go_version} linux/amd64" ] ; then
            echo "Go ${go_version} already installed"
            return
        fi
    fi

	wget -q -O /tmp/go.tar.gz ${download}

	sudo tar -C /tmp -xf /tmp/go.tar.gz
	sudo mv /tmp/go /usr/local
	sudo chown -R root:root /usr/local/go
}

install_go
	
# Ensure that the GOPATH tree is owned by vagrant:vagrant
sudo mkdir -p /opt/gopath
sudo chown -R vagrant:vagrant /opt/gopath

# Ensure new sessions know about GOPATH
sudo tee /etc/profile.d/gopath.sh <<EOT
export GOPATH="/opt/gopath"
export PATH="/usr/local/go/bin:/opt/gopath/bin:\$PATH"
EOT
sudo chmod 755 /etc/profile.d/gopath.sh
