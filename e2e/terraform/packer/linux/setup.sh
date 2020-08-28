#!/bin/bash

set -e

# Disable interactive apt prompts
export DEBIAN_FRONTEND=noninteractive
echo 'debconf debconf/frontend select Noninteractive' | sudo debconf-set-selections


sudo mkdir -p /ops/shared
sudo chown -R ubuntu:ubuntu /ops/shared
cd /ops

CONSULVERSION=1.7.3
CONSULDOWNLOAD=https://releases.hashicorp.com/consul/${CONSULVERSION}/consul_${CONSULVERSION}_linux_amd64.zip
CONSULCONFIGDIR=/etc/consul.d
CONSULDIR=/opt/consul
VAULTVERSION=1.1.1
VAULTDOWNLOAD=https://releases.hashicorp.com/vault/${VAULTVERSION}/vault_${VAULTVERSION}_linux_amd64.zip
VAULTCONFIGDIR=/etc/vault.d
VAULTDIR=/opt/vault

# Will be overwritten by sha specified
NOMADVERSION=0.9.1
NOMADCONFIGDIR=/etc/nomad.d
NOMADDIR=/opt/nomad
NOMADPLUGINDIR=/opt/nomad/plugins

# Dependencies
sudo apt-get update
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
curl -fsL -o /tmp/consul.zip $CONSULDOWNLOAD
sudo unzip -q /tmp/consul.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/consul
sudo chown root:root /usr/local/bin/consul

echo "Configure Consul"
sudo mkdir -p $CONSULCONFIGDIR
sudo chmod 755 $CONSULCONFIGDIR
sudo mkdir -p $CONSULDIR
sudo chmod 755 $CONSULDIR
sudo mv /tmp/consul.service /etc/systemd/system/consul.service

echo "Install Vault"
curl -fsL -o /tmp/vault.zip $VAULTDOWNLOAD
sudo unzip -q /tmp/vault.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/vault
sudo chown root:root /usr/local/bin/vault

echo "Configure Vault"
sudo mkdir -p $VAULTCONFIGDIR
sudo chmod 755 $VAULTCONFIGDIR
sudo mkdir -p $VAULTDIR
sudo chmod 755 $VAULTDIR
sudo mv /tmp/vault.service /etc/systemd/system/vault.service

echo "Configure Nomad"
sudo mkdir -p $NOMADCONFIGDIR
sudo chmod 755 $NOMADCONFIGDIR
sudo mkdir -p $NOMADDIR
sudo chmod 755 $NOMADDIR
sudo mkdir -p $NOMADPLUGINDIR
sudo chmod 755 $NOMADPLUGINDIR
sudo mv /tmp/nomad.service /etc/systemd/system/nomad.service

echo "Install Nomad"
sudo mv /tmp/install-nomad /opt/install-nomad
sudo chmod +x /opt/install-nomad
/opt/install-nomad --nomad_version $NOMADVERSION --nostart

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
JAVA_HOME=$(readlink -f /usr/bin/java | sed "s:bin/java::")

echo "Installing CNI plugins"
sudo mkdir -p /opt/cni/bin
wget -q -O - \
     https://github.com/containernetworking/plugins/releases/download/v0.8.6/cni-plugins-linux-amd64-v0.8.6.tgz \
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

wget -q -P /tmp https://releases.hashicorp.com/nomad-driver-podman/${latest_podman}/nomad-driver-podman_${latest_podman}_linux_amd64.zip
sudo unzip -q /tmp/nomad-driver-podman_${latest_podman}_linux_amd64.zip -d $NOMADPLUGINDIR
sudo chmod +x $NOMADPLUGINDIR/nomad-driver-podman

# enable varlink socket (not included in ubuntu package)
sudo tee /etc/systemd/system/io.podman.service << EOF
[Unit]
Description=Podman Remote API Service
Requires=io.podman.socket
After=io.podman.socket
Documentation=man:podman-varlink(1)

[Service]
Type=simple
ExecStart=/usr/bin/podman varlink unix:%t/podman/io.podman --timeout=60000
TimeoutStopSec=30
KillMode=process

[Install]
WantedBy=multi-user.target
Also=io.podman.socket
EOF

sudo tee /etc/systemd/system/io.podman.socket << EOF
[Unit]
Description=Podman Remote API Socket
Documentation=man:podman-varlink(1) https://podman.io/blogs/2019/01/16/podman-varlink.html

[Socket]
ListenStream=%t/podman/io.podman
SocketMode=0600

[Install]
WantedBy=sockets.target
EOF

# disable systemd-resolved and configure dnsmasq to forward local requests to
# consul. the resolver files need to dynamic configuration based on the VPC
# address and docker bridge IP, so those will be rewritten at boot time.
sudo systemctl disable systemd-resolved.service
echo '
port=53
resolv-file=/var/run/dnsmasq/resolv.conf
bind-interfaces
interface=docker0
interface=lo
interface=eth0
listen-address=127.0.0.1
server=/consul/127.0.0.1#8600
' | sudo tee /etc/dnsmasq.d/default

# this is going to be overwritten at provisioning time, but we need something
# here or we can't fetch binaries to do the provisioning
echo 'nameserver 8.8.8.8' > /tmp/resolv.conf
sudo mv /tmp/resolv.conf /etc/resolv.conf

sudo systemctl restart dnsmasq

# enable cgroup_memory and swap
sudo sed -i 's/GRUB_CMDLINE_LINUX="[^"]*/& cgroup_enable=memory swapaccount=1/' /etc/default/grub
sudo update-grub

echo "Configure user shell"
sudo tee -a /home/ubuntu/.bashrc << 'EOF'
IP_ADDRESS=$(/usr/local/bin/sockaddr eval 'GetPrivateIP')
export CONSUL_RPC_ADDR=$IP_ADDRESS:8400
export CONSUL_HTTP_ADDR=$IP_ADDRESS:8500
export VAULT_ADDR=http://$IP_ADDRESS:8200
export NOMAD_ADDR=http://$IP_ADDRESS:4646
export JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64/jre

EOF
