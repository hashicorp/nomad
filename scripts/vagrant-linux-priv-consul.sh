#!/usr/bin/env bash

set -o errexit

VERSION="1.6.4"
DOWNLOAD=https://releases.hashicorp.com/consul/${VERSION}/consul_${VERSION}_linux_amd64.zip

function install_consul() {
	if [[ -e /usr/bin/consul ]] ; then
		if [ "v${VERSION}" == "$(consul version | head -n1 | awk '{print $2}')" ] ; then
			return
		fi
	fi

	curl -sSL --fail -o /tmp/consul.zip ${DOWNLOAD}

	unzip -d /tmp /tmp/consul.zip
	mv /tmp/consul /usr/bin/consul
	chmod +x /usr/bin/consul
}

install_consul
