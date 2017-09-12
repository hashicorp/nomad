#!/usr/bin/env bash

set -o errexit

sudo apt-get install -y \
	avahi-daemon \
	avahi-discover \
	avahi-utils \
	libnss-mdns \
	mdns-scan
