#!/bin/bash

CONFIGDIR=/ops/shared/config
CONSULCONFIGDIR=/etc/consul.d
NOMADCONFIGDIR=/etc/nomad.d
HOME_DIR=ubuntu

IP_ADDRESS=$(curl http://instance-data/latest/meta-data/local-ipv4)
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
sed -i "s/SERVER_IP_ADDRESS/$SERVER_IP_ADDRESS/g" $CONFIGDIR/nomad_client.hcl
sed -i "s/IP_ADDRESS/$IP_ADDRESS/g" $CONFIGDIR/nomad_client.hcl
sudo cp $CONFIGDIR/nomad_client.hcl $NOMADCONFIGDIR/nomad.hcl
sudo cp $CONFIGDIR/nomad_upstart.conf /etc/init/nomad.conf

sudo service nomad start
sleep 10
export NOMAD_ADDR=http://$IP_ADDRESS:4646

# Set env vars in bashrc

echo "export VAULT_ADDR=http://$IP_ADDRESS:8200" | sudo tee --append /home/$HOME_DIR/.bashrc
echo "export NOMAD_ADDR=http://$IP_ADDRESS:4646" | sudo tee --append /home/$HOME_DIR/.bashrc
echo "export JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64/jre"  | sudo tee --append /home/$HOME_DIR/.bashrc

# Move examples directory to $HOME

sudo mv /ops/examples /home/$HOME_DIR
sudo chown -R $HOME_DIR:$HOME_DIR /home/$HOME_DIR/examples
sudo chmod -R 775 /home/$HOME_DIR/examples

# Copy transcode.sh to /usr/bin
# sudo cp /home/$HOME_DIR/examples/nomad/dispatch/bin/transcode.sh /usr/bin/transcode.sh
