#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -o errexit

apt-get install -y \
	avahi-daemon \
	avahi-discover \
	avahi-utils \
	libnss-mdns \
	mdns-scan
