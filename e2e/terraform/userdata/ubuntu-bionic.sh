#!/bin/bash
# Tasks in the Linux userdata can't be executed during AMI builds because they
# rely on instance-specific data, but they only need to be executed once per
# instance and not on every provisioning of Nomad

# Add hostname to /etc/hosts
echo "127.0.0.1 $(hostname)" | sudo tee --append /etc/hosts

# Use dnsmasq first and then docker bridge network for DNS resolution
DOCKER_BRIDGE_IP_ADDRESS=$(/usr/local/bin/sockaddr eval 'GetInterfaceIP "docker0"')
cat <<EOF > /tmp/resolv.conf
nameserver 127.0.0.1
nameserver $DOCKER_BRIDGE_IP_ADDRESS
EOF
sudo mv /tmp/resolv.conf /etc/resolv.conf

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
sudo mv /tmp/dnsmasq /etc/dnsmasq.d/default

# need to get the AWS DNS address from the VPC...
# this is pretty hacky but will work for any typical case
MAC=$(curl -s --fail http://169.254.169.254/latest/meta-data/mac)
CIDR_BLOCK=$(curl -s --fail "http://169.254.169.254/latest/meta-data/network/interfaces/macs/$MAC/vpc-ipv4-cidr-block")
VPC_DNS_ROOT=$(echo "$CIDR_BLOCK" | cut -d'.' -f1-3)
echo "nameserver ${VPC_DNS_ROOT}.2" > /tmp/dnsmasq-resolv.conf
sudo mv /tmp/dnsmasq-resolv.conf /var/run/dnsmasq/resolv.conf

sudo systemctl restart dnsmasq
sudo systemctl restart docker
