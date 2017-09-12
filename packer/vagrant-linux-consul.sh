#!/usr/bin/env bash

set -o errexit

function install_consul() {
    local version=0.9.2
    local download="https://releases.hashicorp.com/consul/${version}/consul_${version}_linux_amd64.zip"

	if [[ -e /usr/bin/consul ]] ; then
		if [ "v${version}" == "$(consul version | head -n1 | awk '{print $2}')" ] ; then
            echo "Consul ${version} already installed"
			return
		fi
	fi
	
	wget -q -O /tmp/consul.zip ${download}

	unzip -d /tmp /tmp/consul.zip
    sudo install /tmp/consul /usr/bin/consul
}

install_consul
