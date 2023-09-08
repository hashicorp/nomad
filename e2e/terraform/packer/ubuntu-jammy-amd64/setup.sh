#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# setup script for Ubuntu Linux 22.04. Assumes that Packer has placed
# build-time config files at /tmp/linux

set -euo pipefail

NOMAD_PLUGIN_DIR=/opt/nomad/plugins/

mkdir_for_root() {
    sudo mkdir -p "$1"
    sudo chmod 755 "$1"
}

# Disable interactive apt prompts
export DEBIAN_FRONTEND=noninteractive
echo 'debconf debconf/frontend select Noninteractive' | sudo debconf-set-selections

mkdir_for_root /opt
mkdir_for_root /srv/data # for host volumes

# Dependencies
sudo apt-get update
sudo apt-get upgrade -y
sudo apt-get install -y \
     software-properties-common \
     dnsmasq unzip tree redis-tools jq curl tmux awscli nfs-common \
     apt-transport-https ca-certificates gnupg2

# Install hc-install
curl -o /tmp/hc-install.zip https://releases.hashicorp.com/hc-install/0.5.2/hc-install_0.5.2_linux_amd64.zip
sudo unzip -d /usr/local/bin /tmp/hc-install.zip

# Install sockaddr
aws s3 cp "s3://nomad-team-dev-test-binaries/tools/sockaddr_linux_amd64" /tmp/sockaddr
sudo mv /tmp/sockaddr /usr/local/bin
sudo chmod +x /usr/local/bin/sockaddr
sudo chown root:root /usr/local/bin/sockaddr

# Disable the firewall
sudo ufw disable || echo "ufw not installed"

echo "Install HashiCorp apt repositories"
wget -O- https://apt.releases.hashicorp.com/gpg | sudo gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/hashicorp.list

echo "Installing Docker apt repositories"
sudo install -m 0755 -d /etc/apt/keyrings
curl --insecure -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg
echo \
  "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

echo "Refresh apt with third party repositories"
sudo apt-get update

echo "Install Consul and Nomad"
sudo apt-get install -y \
     consul-enterprise \
     nomad

# Note: neither service will start on boot because we haven't enabled
# the systemd unit file and we haven't uploaded any configuration
# files for Consul and Nomad

echo "Configure Consul"
mkdir_for_root /etc/consul.d
mkdir_for_root /opt/consul
sudo mv /tmp/linux/consul.service /etc/systemd/system/consul.service

echo "Configure Nomad"
mkdir_for_root /etc/nomad.d
mkdir_for_root /opt/nomad
mkdir_for_root $NOMAD_PLUGIN_DIR
sudo mv /tmp/linux/nomad.service /etc/systemd/system/nomad.service

echo "Installing third-party tools"

# Docker
echo "Installing Docker CE"
sudo apt-get install -y docker-ce docker-ce-cli

# Java
echo "Installing Java"
sudo apt-get install -y openjdk-17-jdk-headless

# CNI
echo "Installing CNI plugins"
sudo mkdir -p /opt/cni/bin
wget -q -O - \
     https://github.com/containernetworking/plugins/releases/download/v1.0.0/cni-plugins-linux-amd64-v1.0.0.tgz \
    | sudo tar -C /opt/cni/bin -xz

# Podman
echo "Installing Podman"
sudo apt-get -y install podman catatonit

echo "Installing Podman Driver"
sudo hc-install install --path ${NOMAD_PLUGIN_DIR} --version 0.5.0 nomad-driver-podman

# Pledge
echo "Installing Pledge Driver"
curl -k -fsSL -o /tmp/pledge-driver.tar.gz https://github.com/shoenig/nomad-pledge-driver/releases/download/v0.2.3/nomad-pledge-driver_0.2.3_linux_amd64.tar.gz
curl -k -fsSL -o /tmp/pledge https://github.com/shoenig/nomad-pledge-driver/releases/download/pledge-1.8.com/pledge-1.8.com
tar -C /tmp -xf /tmp/pledge-driver.tar.gz
sudo mv /tmp/nomad-pledge-driver ${NOMAD_PLUGIN_DIR}
sudo mv /tmp/pledge /usr/local/bin
sudo chmod +x /usr/local/bin/pledge

# ECS
if [ -a "/tmp/linux/nomad-driver-ecs" ]; then
    echo "Installing nomad-driver-ecs"
    sudo install --mode=0755 --owner=ubuntu /tmp/linux/nomad-driver-ecs "$NOMAD_PLUGIN_DIR"
else
    echo "nomad-driver-ecs not found: skipping install"
fi

echo "Configuring dnsmasq"

# disable systemd stub resolver
sudo sed -i 's|#DNSStubListener=yes|DNSStubListener=no|g' /etc/systemd/resolved.conf

# disable systemd-resolved and configure dnsmasq to forward local requests to
# consul. the resolver files need to dynamic configuration based on the VPC
# address and docker bridge IP, so those will be rewritten at boot time.
sudo systemctl disable systemd-resolved.service
sudo systemctl stop systemd-resolved.service
sudo mv /tmp/linux/dnsmasq /etc/dnsmasq.d/default
sudo chown root:root /etc/dnsmasq.d/default

# this is going to be overwritten at provisioning time, but we need something
# here or we can't fetch binaries to do the provisioning
echo 'nameserver 8.8.8.8' > /tmp/resolv.conf
sudo mv /tmp/resolv.conf /etc/resolv.conf

sudo mv /tmp/linux/dnsmasq.service /etc/systemd/system/dnsmasq.service
sudo mv /tmp/linux/dnsconfig.sh /usr/local/bin/dnsconfig.sh
sudo chmod +x /usr/local/bin/dnsconfig.sh
sudo systemctl daemon-reload

echo "Updating boot parameters"

# enable cgroup_memory and swap
sudo sed -i 's/GRUB_CMDLINE_LINUX="[^"]*/& cgroup_enable=memory swapaccount=1/' /etc/default/grub
sudo update-grub

echo "Configuring user shell"
sudo tee -a /home/ubuntu/.bashrc << 'EOF'
IP_ADDRESS=$(/usr/local/bin/sockaddr eval 'GetPrivateIP')
export CONSUL_RPC_ADDR=$IP_ADDRESS:8400
export CONSUL_HTTP_ADDR=$IP_ADDRESS:8500
export VAULT_ADDR=http://$IP_ADDRESS:8200
export NOMAD_ADDR=http://$IP_ADDRESS:4646
export JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64/bin

EOF
