#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -o errexit

apt-get install -y \
	avahi-daemon \
	avahi-discover \
	avahi-utils \
	libnss-mdns \
	mdns-scan
