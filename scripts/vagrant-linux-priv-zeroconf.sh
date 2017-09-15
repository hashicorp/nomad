#!/usr/bin/env bash

set -o errexit

apt-get install -y \
	avahi-daemon \
	avahi-discover \
	avahi-utils \
	libnss-mdns \
	mdns-scan
