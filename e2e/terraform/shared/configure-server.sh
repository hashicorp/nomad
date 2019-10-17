#!/bin/bash

set -e

cfg=/opt/shared
CONFIGDIR=/opt/shared/config

VAULTCONFIGDIR=/etc/vault.d
NOMADCONFIGDIR=/etc/nomad.d
HADOOP_VERSION=hadoop-2.7.6
HADOOPCONFIGDIR=/usr/local/$HADOOP_VERSION/etc/hadoop
HOME_DIR=ubuntu

# Wait for network
sleep 15

# IP_ADDRESS=$(curl http://instance-data/latest/meta-data/local-ipv4)
IP_ADDRESS="$(/sbin/ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')"
DOCKER_BRIDGE_IP_ADDRESS=(`ifconfig docker0 2>/dev/null|awk '/inet addr:/ {print $2}'|sed 's/addr://'`)
CLOUD=$1
SERVER_COUNT=$2

# Consul
sed "s/SERVER_COUNT/${SERVER_COUNT}/g" \
    "$cfg/consul/consul_server_${CLOUD}.json" > /etc/consul.d/consul.json
sudo cp "$cfg/consul/consul_${CLOUD}.service" /etc/systemd/system/consul.service
sudo systemctl enable consul.service
sudo systemctl start consul.service

sleep 10
export CONSUL_HTTP_ADDR=$IP_ADDRESS:8500
export CONSUL_RPC_ADDR=$IP_ADDRESS:8400

# Vault
sudo cp "$cfg/vault/vault.hcl" /etc/vault.d/vault.hcl
sudo cp "$cfg/vault/vault.service" /etc/systemd/system/vault.service

sudo systemctl enable vault.service
sudo systemctl start vault.service

export NOMAD_ADDR=http://$IP_ADDRESS:4646

# Add hostname to /etc/hosts
echo "127.0.0.1 $(hostname)" | sudo tee --append /etc/hosts

# Add Docker bridge network IP to /etc/resolv.conf (at the top)

echo "nameserver $DOCKER_BRIDGE_IP_ADDRESS" | sudo tee /etc/resolv.conf.new
cat /etc/resolv.conf | sudo tee --append /etc/resolv.conf.new
sudo mv /etc/resolv.conf.new /etc/resolv.conf

# Hadoop
sudo cp "$cfg/spark/core-site.xml" $HADOOPCONFIGDIR

# Move examples directory to $HOME
sudo mv /opt/shared/examples /home/$HOME_DIR
sudo chown -R $HOME_DIR:$HOME_DIR /home/$HOME_DIR/examples
sudo chmod -R 775 /home/$HOME_DIR/examples

# Set env vars for tool CLIs
echo "export CONSUL_RPC_ADDR=$IP_ADDRESS:8400" | sudo tee --append /home/$HOME_DIR/.bashrc
echo "export CONSUL_HTTP_ADDR=$IP_ADDRESS:8500" | sudo tee --append /home/$HOME_DIR/.bashrc
echo "export VAULT_ADDR=http://$IP_ADDRESS:8200" | sudo tee --append /home/$HOME_DIR/.bashrc
echo "export NOMAD_ADDR=http://$IP_ADDRESS:4646" | sudo tee --append /home/$HOME_DIR/.bashrc
echo "export JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64/jre"  | sudo tee --append /home/$HOME_DIR/.bashrc

# Update PATH
echo "export PATH=$PATH:/usr/local/bin/spark/bin:/usr/local/$HADOOP_VERSION/bin" | sudo tee --append /home/$HOME_DIR/.bashrc
