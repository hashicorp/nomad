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
NOMADDOWNLOAD=https://releases.hashicorp.com/nomad/${NOMADVERSION}/nomad_${NOMADVERSION}_linux_amd64.zip
NOMADCONFIGDIR=/etc/nomad.d
NOMADDIR=/opt/nomad
NOMADPLUGINDIR=/opt/nomad/plugins

# Dependencies
sudo apt-get install -y software-properties-common
sudo apt-get update
sudo apt-get install -y dnsmasq unzip tree redis-tools jq curl tmux awscli nfs-common

# Numpy (for Spark)
sudo apt-get install -y python-setuptools python-pip
sudo pip install numpy

# Install sockaddr
aws s3 cp "s3://nomad-team-test-binary/tools/sockaddr_linux_amd64" /tmp/sockaddr
sudo mv /tmp/sockaddr /usr/local/bin
sudo chmod +x /usr/local/bin/sockaddr
sudo chown root:root /usr/local/bin/sockaddr

# Disable the firewall
sudo ufw disable || echo "ufw not installed"

echo "Install Consul"
curl -L -o /tmp/consul.zip $CONSULDOWNLOAD
sudo unzip /tmp/consul.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/consul
sudo chown root:root /usr/local/bin/consul

echo "Configure Consul"
sudo mkdir -p $CONSULCONFIGDIR
sudo chmod 755 $CONSULCONFIGDIR
sudo mkdir -p $CONSULDIR
sudo chmod 755 $CONSULDIR

echo "Install Vault"
curl -L -o /tmp/vault.zip $VAULTDOWNLOAD
sudo unzip /tmp/vault.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/vault
sudo chown root:root /usr/local/bin/vault

echo "Configure Vault"
sudo mkdir -p $VAULTCONFIGDIR
sudo chmod 755 $VAULTCONFIGDIR
sudo mkdir -p $VAULTDIR
sudo chmod 755 $VAULTDIR

echo "Install Nomad"
curl -L -o /tmp/nomad.zip $NOMADDOWNLOAD
sudo unzip /tmp/nomad.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/nomad
sudo chown root:root /usr/local/bin/nomad

echo "Configure Nomad"
sudo mkdir -p $NOMADCONFIGDIR
sudo chmod 755 $NOMADCONFIGDIR
sudo mkdir -p $NOMADDIR
sudo chmod 755 $NOMADDIR
sudo mkdir -p $NOMADPLUGINDIR
sudo chmod 755 $NOMADPLUGINDIR

echo "Install Docker"
distro=$(lsb_release -si | tr '[:upper:]' '[:lower:]')
sudo apt-get install -y apt-transport-https ca-certificates gnupg2
curl -fsSL https://download.docker.com/linux/debian/gpg | sudo apt-key add -
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/${distro} $(lsb_release -cs) stable"
sudo apt-get update
sudo apt-get install -y docker-ce

echo "Install Java"
sudo add-apt-repository -y ppa:openjdk-r/ppa
sudo apt-get update
sudo apt-get install -y openjdk-8-jdk
JAVA_HOME=$(readlink -f /usr/bin/java | sed "s:bin/java::")

echo "Install Spark"
sudo wget -P /ops/examples/spark https://nomad-spark.s3.amazonaws.com/spark-2.2.0-bin-nomad-0.7.0.tgz
sudo tar -xvf /ops/examples/spark/spark-2.2.0-bin-nomad-0.7.0.tgz --directory /ops/examples/spark
sudo mv /ops/examples/spark/spark-2.2.0-bin-nomad-0.7.0 /usr/local/bin/spark
sudo chown -R root:root /usr/local/bin/spark

echo "Install HDFS CLI"
HADOOP_VERSION=2.7.7
HADOOPCONFIGDIR=/usr/local/$HADOOP_VERSION/etc/hadoop
sudo mkdir -p "$HADOOPCONFIGDIR"

wget -O - http://apache.mirror.iphh.net/hadoop/common/hadoop-${HADOOP_VERSION}/hadoop-${HADOOP_VERSION}.tar.gz | sudo tar xz -C /usr/local/

echo "Install Podman"
. /etc/os-release
sudo sh -c "echo 'deb https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_${VERSION_ID}/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list"
curl -L https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_${VERSION_ID}/Release.key | sudo apt-key add -
sudo apt-get update -qq
sudo apt-get -qq -y install podman 

# get catatonit (to check podman --init switch)
cd /tmp
wget https://github.com/openSUSE/catatonit/releases/download/v0.1.4/catatonit.x86_64
mkdir -p /usr/libexec/podman
sudo mv catatonit* /usr/libexec/podman/catatonit
sudo chmod +x /usr/libexec/podman/catatonit

echo "Install podman task driver"
# install nomad-podman-driver and move to plugin dir
wget -P /tmp https://github.com/pascomnet/nomad-driver-podman/releases/download/v0.0.3/nomad-driver-podman_linux_amd64.tar.gz 
sudo tar -xf /tmp/nomad-driver-podman_linux_amd64.tar.gz -C /tmp
sudo mv /tmp/nomad-driver-podman/nomad-driver-podman $NOMADPLUGINDIR
sudo chmod +x $NOMADPLUGINDIR/nomad-driver-podman

# disable systemd-resolved and configure dnsmasq
# to forward local requests to consul
sudo systemctl disable systemd-resolved.service
sudo rm /etc/resolv.conf
echo "nameserver 8.8.8.8" | sudo tee /etc/resolv.conf
echo '
port=53
resolv-file=/var/run/dnsmasq/resolv.conf
bind-interfaces
listen-address=127.0.0.1
server=/consul/127.0.0.1#8600
' | sudo tee /etc/dnsmasq.d/default

# add our hostname to etc/hosts
echo "127.0.0.1 $(hostname)" | sudo tee -a /etc/hosts
sudo systemctl restart dnsmasq

# enable cgroup_memory and swap
sudo sed -i 's/GRUB_CMDLINE_LINUX="[^"]*/& cgroup_enable=memory swapaccount=1/' /etc/default/grub
sudo update-grub

# note this 'EOF' syntax avoids expansion in the heredoc
sudo tee "$HADOOPCONFIGDIR/core-site.xml" << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<?xml-stylesheet type="text/xsl" href="configuration.xsl"?>
<configuration>
    <property>
        <name>fs.defaultFS</name>
        <value>hdfs://hdfs.service.consul/</value>
    </property>
</configuration>
EOF

echo "Configure user shell"
sudo tee -a /home/ubuntu/.bashrc << 'EOF'
IP_ADDRESS=$(/usr/local/bin/sockaddr eval 'GetPrivateIP')
export CONSUL_RPC_ADDR=$IP_ADDRESS:8400
export CONSUL_HTTP_ADDR=$IP_ADDRESS:8500
export VAULT_ADDR=http://$IP_ADDRESS:8200
export NOMAD_ADDR=http://$IP_ADDRESS:4646
export JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64/jre

# Update PATH
export PATH=$PATH:/usr/local/bin/spark/bin:/usr/local/$HADOOP_VERSION/bin
EOF
