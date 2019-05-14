#!/bin/bash

set -e

# Disable interactive apt prompts
export DEBIAN_FRONTEND=noninteractive

cd /ops

CONFIGDIR=/ops/shared/config

CONSULVERSION=1.4.4
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

HADOOP_VERSION=2.7.6

# Dependencies
sudo apt-get install -y software-properties-common
sudo apt-get update
sudo apt-get install -y unzip tree redis-tools jq curl tmux awscli

# Numpy (for Spark)
sudo apt-get install -y python-setuptools
sudo easy_install pip
sudo pip install numpy

# Disable the firewall

sudo ufw disable || echo "ufw not installed"

echo "Install Consul"
curl -L $CONSULDOWNLOAD > consul.zip
sudo unzip consul.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/consul
sudo chown root:root /usr/local/bin/consul

echo "Configure Consul"
sudo mkdir -p $CONSULCONFIGDIR
sudo chmod 755 $CONSULCONFIGDIR
sudo mkdir -p $CONSULDIR
sudo chmod 755 $CONSULDIR

echo "Install Vault"
curl -L $VAULTDOWNLOAD > vault.zip
sudo unzip vault.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/vault
sudo chown root:root /usr/local/bin/vault

echo "Configure Vault"
sudo mkdir -p $VAULTCONFIGDIR
sudo chmod 755 $VAULTCONFIGDIR
sudo mkdir -p $VAULTDIR
sudo chmod 755 $VAULTDIR

echo "Install Nomad"
curl -L $NOMADDOWNLOAD > nomad.zip
sudo unzip nomad.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/nomad
sudo chown root:root /usr/local/bin/nomad

echo "Configure Nomad"
sudo mkdir -p $NOMADCONFIGDIR
sudo chmod 755 $NOMADCONFIGDIR
sudo mkdir -p $NOMADDIR
sudo chmod 755 $NOMADDIR

# Docker
distro=$(lsb_release -si | tr '[:upper:]' '[:lower:]')
sudo apt-get install -y apt-transport-https ca-certificates gnupg2
curl -fsSL https://download.docker.com/linux/debian/gpg | sudo apt-key add -
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/${distro} $(lsb_release -cs) stable"
sudo apt-get update
sudo apt-get install -y docker-ce

# rkt
VERSION=1.29.0
DOWNLOAD=https://github.com/rkt/rkt/releases/download/v${VERSION}/rkt-v${VERSION}.tar.gz

function install_rkt() {
	wget -q -O /tmp/rkt.tar.gz "${DOWNLOAD}"
	tar -C /tmp -xvf /tmp/rkt.tar.gz
	sudo mv /tmp/rkt-v${VERSION}/rkt /usr/local/bin
	sudo mv /tmp/rkt-v${VERSION}/*.aci /usr/local/bin
}

function configure_rkt_networking() {
	sudo mkdir -p /etc/rkt/net.d
    sudo bash -c 'cat << EOT > /etc/rkt/net.d/99-network.conf
{
  "name": "default",
  "type": "ptp",
  "ipMasq": false,
  "ipam": {
    "type": "host-local",
    "subnet": "172.16.28.0/24",
    "routes": [
      {
        "dst": "0.0.0.0/0"
      }
    ]
  }
}
EOT'
}

install_rkt
configure_rkt_networking

# Java
sudo add-apt-repository -y ppa:openjdk-r/ppa
sudo apt-get update
sudo apt-get install -y openjdk-8-jdk
JAVA_HOME=$(readlink -f /usr/bin/java | sed "s:bin/java::")

# Spark
sudo wget -P /ops/examples/spark https://nomad-spark.s3.amazonaws.com/spark-2.2.0-bin-nomad-0.7.0.tgz
sudo tar -xvf /ops/examples/spark/spark-2.2.0-bin-nomad-0.7.0.tgz --directory /ops/examples/spark
sudo mv /ops/examples/spark/spark-2.2.0-bin-nomad-0.7.0 /usr/local/bin/spark
sudo chown -R root:root /usr/local/bin/spark

# Hadoop (to enable the HDFS CLI)
wget -O - http://apache.mirror.iphh.net/hadoop/common/hadoop-${HADOOP_VERSION}/hadoop-${HADOOP_VERSION}.tar.gz | sudo tar xz -C /usr/local/
