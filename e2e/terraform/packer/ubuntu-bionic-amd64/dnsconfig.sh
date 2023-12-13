#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -e

# These tasks can't be executed during AMI builds because they rely on
# instance-specific data.

mkdir -p /var/run/dnsmasq
mkdir -p /etc/dnsmasq.d

# Add hostname to /etc/hosts
echo "127.0.0.1 $(hostname)" | tee --append /etc/hosts

# this script should run after docker.service but we can't guarantee
# it's created docker0 yet, so wait to make sure
while ! (ip link | grep -q docker0)
do
    sleep 1
done

# Use dnsmasq first and then docker bridge network for DNS resolution
DOCKER_BRIDGE_IP_ADDRESS=$(/usr/local/bin/sockaddr eval 'GetInterfaceIP "docker0"')
cat <<EOF > /tmp/resolv.conf
nameserver 127.0.0.1
nameserver $DOCKER_BRIDGE_IP_ADDRESS
EOF
cp /tmp/resolv.conf /etc/resolv.conf

# need to get the interface for dnsmasq config so that we can
# accomodate both "predictable" and old-style interface names
IFACE=$(/usr/local/bin/sockaddr eval 'GetDefaultInterfaces | attr "Name"')

cat <<EOF > /tmp/dnsmasq
port=53
resolv-file=/var/run/dnsmasq/resolv.conf
bind-interfaces
interface=docker0
interface=lo
interface=$IFACE
listen-address=127.0.0.1
server=/consul/127.0.0.1#8600
EOF
cp /tmp/dnsmasq /etc/dnsmasq.d/default

# need to get the AWS DNS address from the VPC...
# this is pretty hacky but will work for any typical case
MAC=$(curl -s --fail http://169.254.169.254/latest/meta-data/mac)
CIDR_BLOCK=$(curl -s --fail "http://169.254.169.254/latest/meta-data/network/interfaces/macs/$MAC/vpc-ipv4-cidr-block")
VPC_DNS_ROOT=$(echo "$CIDR_BLOCK" | cut -d'.' -f1-3)
echo "nameserver ${VPC_DNS_ROOT}.2" > /tmp/dnsmasq-resolv.conf
cp /tmp/dnsmasq-resolv.conf /var/run/dnsmasq/resolv.conf

/usr/sbin/dnsmasq --test
