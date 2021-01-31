#!/bin/bash
# setup script for Ubuntu Linux 18.04. Assumes that Packer has placed
# build-time config files at /tmp/linux

set -e

# Will be overwritten at test time with the version specified
NOMADVERSION=0.12.7
CONSULVERSION=1.9.0
VAULTVERSION=1.5.4

NOMAD_PLUGIN_DIR=/opt/nomad/plugins/

mkdir_for_root() {
    sudo mkdir -p "$1"
    sudo chmod 755 "$1"
}

# Disable interactive apt prompts
export DEBIAN_FRONTEND=noninteractive
echo 'debconf debconf/frontend select Noninteractive' | sudo debconf-set-selections

sudo mkdir -p /ops/shared
sudo chown -R ubuntu:ubuntu /ops/shared

mkdir_for_root /opt
mkdir_for_root /srv/data # for host volumes

# Dependencies
sudo apt-get update
sudo apt-get upgrade -y
sudo apt-get install -y \
     software-properties-common \
     dnsmasq unzip tree redis-tools jq curl tmux awscli nfs-common \
     apt-transport-https ca-certificates gnupg2

# Install sockaddr
aws s3 cp "s3://nomad-team-dev-test-binaries/tools/sockaddr_linux_amd64" /tmp/sockaddr
sudo mv /tmp/sockaddr /usr/local/bin
sudo chmod +x /usr/local/bin/sockaddr
sudo chown root:root /usr/local/bin/sockaddr

# Disable the firewall
sudo ufw disable || echo "ufw not installed"

echo "Install Consul"
curl -fsL -o /tmp/consul.zip \
     "https://releases.hashicorp.com/consul/${CONSULVERSION}/consul_${CONSULVERSION}_linux_amd64.zip"
sudo unzip -q /tmp/consul.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/consul
sudo chown root:root /usr/local/bin/consul

echo "Configure Consul"
mkdir_for_root /etc/consul.d
mkdir_for_root /opt/consul
sudo mv /tmp/linux/consul.service /etc/systemd/system/consul.service

echo "Install Vault"
curl -fsL -o /tmp/vault.zip \
     "https://releases.hashicorp.com/vault/${VAULTVERSION}/vault_${VAULTVERSION}_linux_amd64.zip"
sudo unzip -q /tmp/vault.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/vault
sudo chown root:root /usr/local/bin/vault

echo "Configure Vault"
mkdir_for_root /etc/vault.d
mkdir_for_root /opt/vault
sudo mv /tmp/linux/vault.service /etc/systemd/system/vault.service

sudo setcap cap_ipc_lock=+ep /usr/local/bin/vault
sudo useradd --system --home /etc/vault.d --shell /bin/false vault

echo "Configure Nomad"
mkdir_for_root /etc/nomad.d
mkdir_for_root /opt/nomad
mkdir_for_root $NOMAD_PLUGIN_DIR
sudo mv /tmp/linux/nomad.service /etc/systemd/system/nomad.service

echo "Install Nomad"
sudo mv /tmp/linux/provision.sh /opt/provision.sh
sudo chmod +x /opt/provision.sh
/opt/provision.sh --nomad_version $NOMADVERSION --nostart

echo "Installing third-party apt repositories"

# Docker
distro=$(lsb_release -si | tr '[:upper:]' '[:lower:]')
curl -fsSL https://download.docker.com/linux/debian/gpg | sudo apt-key add -
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/${distro} $(lsb_release -cs) stable"

# Java
sudo add-apt-repository -y ppa:openjdk-r/ppa

# Podman
. /etc/os-release
curl -fsSL "https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_${VERSION_ID}/Release.key" | sudo apt-key add -
sudo add-apt-repository "deb https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_${VERSION_ID}/ /"

sudo apt-get update

echo "Installing Docker"
sudo apt-get install -y docker-ce

echo "Installing Java"
sudo apt-get install -y openjdk-8-jdk

echo "Installing CNI plugins"
sudo mkdir -p /opt/cni/bin
wget -q -O - \
     https://github.com/containernetworking/plugins/releases/download/v0.9.0/cni-plugins-linux-amd64-v0.9.0.tgz \
    | sudo tar -C /opt/cni/bin -xz

echo "Installing Podman"
sudo apt-get -y install podman

# get catatonit (to check podman --init switch)
wget -q -P /tmp https://github.com/openSUSE/catatonit/releases/download/v0.1.4/catatonit.x86_64
mkdir -p /usr/libexec/podman
sudo mv /tmp/catatonit* /usr/libexec/podman/catatonit
sudo chmod +x /usr/libexec/podman/catatonit

echo "Installing latest podman task driver"
# install nomad-podman-driver and move to plugin dir
latest_podman=$(curl -s https://releases.hashicorp.com/nomad-driver-podman/index.json | jq --raw-output '.versions |= with_entries(select(.key|match("^\\d+\\.\\d+\\.\\d+$"))) | .versions | keys[]' | sort -rV | head -n1)

wget -q -P /tmp "https://releases.hashicorp.com/nomad-driver-podman/${latest_podman}/nomad-driver-podman_${latest_podman}_linux_amd64.zip"
sudo unzip -q "/tmp/nomad-driver-podman_${latest_podman}_linux_amd64.zip" -d "$NOMAD_PLUGIN_DIR"
sudo chmod +x "${NOMAD_PLUGIN_DIR}/nomad-driver-podman"

# enable varlink socket (not included in ubuntu package)
sudo mv /tmp/linux/io.podman.service /etc/systemd/system/io.podman.service
sudo mv /tmp/linux/io.podman.socket /etc/systemd/system/io.podman.socket

echo "Configuring dnsmasq"

# disable systemd-resolved and configure dnsmasq to forward local requests to
# consul. the resolver files need to dynamic configuration based on the VPC
# address and docker bridge IP, so those will be rewritten at boot time.
sudo systemctl disable systemd-resolved.service
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
export JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64/jre

EOF
