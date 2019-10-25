#!/bin/bash
# installs and configures the desired build of Nomad as a server
set -o errexit
set -o nounset

CONFIGDIR=/ops/shared/config

CONSULCONFIGDIR=/etc/consul.d
NOMADCONFIGDIR=/etc/nomad.d
HADOOP_VERSION=hadoop-2.7.7
HADOOPCONFIGDIR=/usr/local/$HADOOP_VERSION/etc/hadoop
HOME_DIR=ubuntu

IP_ADDRESS=$(/usr/local/bin/sockaddr eval 'GetPrivateIP')
DOCKER_BRIDGE_IP_ADDRESS=$(/usr/local/bin/sockaddr eval 'GetInterfaceIP "docker0"')

CLOUD="$1"
SERVER_COUNT="$2"
RETRY_JOIN="$3"
nomad_sha="$4"

# Consul
sed -i "s/SERVER_COUNT/$SERVER_COUNT/g" $CONFIGDIR/consul.json
sed -i "s/RETRY_JOIN/$RETRY_JOIN/g" $CONFIGDIR/consul.json
sudo cp $CONFIGDIR/consul.json $CONSULCONFIGDIR
sudo cp $CONFIGDIR/consul_$CLOUD.service /etc/systemd/system/consul.service

sudo systemctl enable consul.service
sudo systemctl start  consul.service
sleep 10
export CONSUL_HTTP_ADDR=$IP_ADDRESS:8500
export CONSUL_RPC_ADDR=$IP_ADDRESS:8400

# Vault
VAULT_SRC=/ops/shared/vault
VAULT_DEST=/etc/vault.d

sudo cp "$VAULT_SRC/vault.hcl" "$VAULT_DEST"
sudo cp "$VAULT_SRC/vault.service" /etc/systemd/system/vault.service

sudo systemctl enable vault.service
sudo systemctl start  vault.service

export NOMAD_ADDR=http://$IP_ADDRESS:4646

# Add hostname to /etc/hosts
echo "127.0.0.1 $(hostname)" | sudo tee --append /etc/hosts

# Add Docker bridge network IP to /etc/resolv.conf (at the top)

echo "nameserver $DOCKER_BRIDGE_IP_ADDRESS" | sudo tee /etc/resolv.conf.new
cat /etc/resolv.conf | sudo tee --append /etc/resolv.conf.new
sudo mv /etc/resolv.conf.new /etc/resolv.conf

# Hadoop
sudo cp $CONFIGDIR/core-site.xml $HADOOPCONFIGDIR

# Move examples directory to $HOME
sudo mv /ops/examples /home/$HOME_DIR
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

# download
aws s3 cp s3://nomad-team-test-binary/builds-oss/${nomad_sha}.tar.gz nomad.tar.gz

# unpack and install
sudo tar -zxvf nomad.tar.gz -C /usr/local/bin/
sudo chmod 0755 /usr/local/bin/nomad
sudo chown root:root /usr/local/bin/nomad

# install config file
sudo cp /tmp/server.hcl /etc/nomad.d/nomad.hcl

# enable as a systemd service
sudo cp /ops/shared/config/nomad.service /etc/systemd/system/nomad.service
sudo systemctl enable nomad.service
sudo systemctl start nomad.service
