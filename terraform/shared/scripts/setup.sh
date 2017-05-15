#!/bin/bash
set -e

cd /ops

CONFIGDIR=/ops/shared/config

CONSULVERSION=0.7.5
CONSULDOWNLOAD=https://releases.hashicorp.com/consul/${CONSULVERSION}/consul_${CONSULVERSION}_linux_amd64.zip
CONSULCONFIGDIR=/etc/consul.d
CONSULDIR=/opt/consul

VAULTVERSION=0.6.5
VAULTDOWNLOAD=https://releases.hashicorp.com/vault/${VAULTVERSION}/vault_${VAULTVERSION}_linux_amd64.zip
VAULTCONFIGDIR=/etc/vault.d
VAULTDIR=/opt/vault

NOMADVERSION=0.5.6
NOMADDOWNLOAD=https://releases.hashicorp.com/nomad/${NOMADVERSION}/nomad_${NOMADVERSION}_linux_amd64.zip
NOMADCONFIGDIR=/etc/nomad.d
NOMADDIR=/opt/nomad

PACKERVERSION=0.12.3
PACKERDOWNLOAD=https://releases.hashicorp.com/packer/${PACKERVERSION}/packer_${PACKERVERSION}_linux_amd64.zip

echo Dependencies...
sudo apt-get install -y software-properties-common
sudo add-apt-repository ppa:mc3man/trusty-media
sudo apt-get update
sudo apt-get install -y unzip tree redis-tools jq s3cmd ffmpeg

# Numpy

sudo apt-get install -y python-setuptools
sudo easy_install pip
sudo pip install numpy

# Instead of symlink, move ffmpeg to be inside the chroot for Nomad
sudo rm /usr/bin/ffmpeg
sudo cp /opt/ffmpeg/bin/ffmpeg /usr/bin/ffmpeg

# Disable the firewall

sudo ufw disable

## Consul

echo Fetching Consul...
curl -L $CONSULDOWNLOAD > consul.zip

echo Installing Consul...
sudo unzip consul.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/consul
sudo chown root:root /usr/local/bin/consul

echo Configuring Consul...
sudo mkdir -p $CONSULCONFIGDIR
sudo chmod 755 $CONSULCONFIGDIR
sudo mkdir -p $CONSULDIR
sudo chmod 755 $CONSULDIR

## Vault

echo Fetching Vault...
curl -L $VAULTDOWNLOAD > vault.zip

echo Installing Vault...
sudo unzip vault.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/vault
sudo chown root:root /usr/local/bin/vault

echo Configuring Vault...
sudo mkdir -p $VAULTCONFIGDIR
sudo chmod 755 $VAULTCONFIGDIR
sudo mkdir -p $VAULTDIR
sudo chmod 755 $VAULTDIR

## Nomad

echo Fetching Nomad...
curl -L $NOMADDOWNLOAD > nomad.zip

echo Installing Nomad...
sudo unzip nomad.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/nomad
sudo chown root:root /usr/local/bin/nomad

echo Configuring Nomad...
sudo mkdir -p $NOMADCONFIGDIR
sudo chmod 755 $NOMADCONFIGDIR
sudo mkdir -p $NOMADDIR
sudo chmod 755 $NOMADDIR

## Packer

echo Fetching Packer...
curl -L $PACKERDOWNLOAD > packer.zip

echo Installing Packer...
sudo unzip packer.zip -d /usr/local/bin
sudo chmod 0755 /usr/local/bin/packer
sudo chown root:root /usr/local/bin/packer

## Docker

echo deb https://apt.dockerproject.org/repo ubuntu-`lsb_release -c | awk '{print $2}'` main | sudo tee /etc/apt/sources.list.d/docker.list
sudo apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D
sudo apt-get update
sudo apt-get install -y docker-engine

## Java

sudo add-apt-repository -y ppa:openjdk-r/ppa
sudo apt-get update 
sudo apt-get install -y openjdk-8-jdk
JAVA_HOME=$(readlink -f /usr/bin/java | sed "s:bin/java::")

## Download and unpack spark

sudo wget -P /ops/examples/spark https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz
sudo tar -xvf /ops/examples/spark/spark-2.1.0-bin-nomad-preview-6.tgz --directory /ops/examples/spark
sudo mv /ops/examples/spark/spark-2.1.0-bin-nomad-preview-6 /ops/examples/spark/spark
sudo rm /ops/examples/spark/spark-2.1.0-bin-nomad-preview-6.tgz

