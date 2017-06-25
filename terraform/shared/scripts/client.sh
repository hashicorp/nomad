#!/bin/bash

set -e

CONFIGDIR=/ops/shared/config

CONSULCONFIGDIR=/etc/consul.d
NOMADCONFIGDIR=/etc/nomad.d
HADOOP_VERSION=hadoop-2.7.3
HADOOPCONFIGDIR=/usr/local/$HADOOP_VERSION/etc/hadoop
HOME_DIR=ubuntu

# Wait for network
sleep 15

IP_ADDRESS=$(curl http://instance-data/latest/meta-data/local-ipv4)
DOCKER_BRIDGE_IP_ADDRESS=(`ifconfig docker0 2>/dev/null|awk '/inet addr:/ {print $2}'|sed 's/addr://'`)
REGION=$1
CLUSTER_TAG_VALUE=$2

# Consul
sed -i "s/IP_ADDRESS/$IP_ADDRESS/g" $CONFIGDIR/consul_client.json
sed -i "s/REGION/$REGION/g" $CONFIGDIR/consul_client.json
sed -i "s/CLUSTER_TAG_VALUE/$CLUSTER_TAG_VALUE/g" $CONFIGDIR/consul_client.json
sudo cp $CONFIGDIR/consul_client.json $CONSULCONFIGDIR/consul.json
sudo cp $CONFIGDIR/consul_upstart.conf /etc/init/consul.conf

sudo service consul start
sleep 10

# Nomad
sed -i "s/IP_ADDRESS/$IP_ADDRESS/g" $CONFIGDIR/nomad_client.hcl
sudo cp $CONFIGDIR/nomad_client.hcl $NOMADCONFIGDIR/nomad.hcl
sudo cp $CONFIGDIR/nomad_upstart.conf /etc/init/nomad.conf

sudo service nomad start
sleep 10
export NOMAD_ADDR=http://$IP_ADDRESS:4646

# Add hostname to /etc/hosts
echo "127.0.0.1 $(hostname)" | sudo tee --append /etc/hosts

# Add Docker bridge network IP to /etc/resolv.conf (at the top)
echo "nameserver $DOCKER_BRIDGE_IP_ADDRESS" | sudo tee /etc/resolv.conf.new
cat /etc/resolv.conf | sudo tee --append /etc/resolv.conf.new
sudo mv /etc/resolv.conf.new /etc/resolv.conf

# Hadoop config file to enable HDFS CLI
sudo cp $CONFIGDIR/core-site.xml $HADOOPCONFIGDIR

# Move examples directory to $HOME
sudo mv /ops/examples /home/$HOME_DIR
sudo chown -R $HOME_DIR:$HOME_DIR /home/$HOME_DIR/examples
sudo chmod -R 775 /home/$HOME_DIR/examples

# Set env vars for tool CLIs
echo "export VAULT_ADDR=http://$IP_ADDRESS:8200" | sudo tee --append /home/$HOME_DIR/.bashrc
echo "export NOMAD_ADDR=http://$IP_ADDRESS:4646" | sudo tee --append /home/$HOME_DIR/.bashrc
echo "export JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64/jre"  | sudo tee --append /home/$HOME_DIR/.bashrc

# Update PATH
echo "export PATH=$PATH:/usr/local/bin/spark/bin:/usr/local/$HADOOP_VERSION/bin" | sudo tee --append /home/$HOME_DIR/.bashrc


