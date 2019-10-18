#!/bin/bash
# installs the desired Nomad build and configures machine as a Nomad server
set -o errexit
set -o nounset

cloud=$1
nomad_sha=$2
server_count=$3

cfg=/opt/shared
home_dir=/home/ubuntu

# ------------------------
# Networking for containers

ip_address=$(/usr/local/bin/sockaddr eval 'GetPrivateIP')
docker_bridge_ip=$(/usr/local/bin/sockaddr eval 'GetInterfaceIP "docker0"')

# Add hostname to /etc/hosts
echo "127.0.0.1 $(hostname)" | sudo tee --append /etc/hosts

# Add Docker bridge network IP to /etc/resolv.conf (at the top)
echo "nameserver $docker_bridge_ip" | sudo tee /etc/resolv.conf.new
cat /etc/resolv.conf | sudo tee --append /etc/resolv.conf.new
sudo mv /etc/resolv.conf.new /etc/resolv.conf

# ------------------------
# Consul

sed "s/SERVER_COUNT/${server_count}/g" \
    "$cfg/consul/consul_server_${cloud}.json" > /etc/consul.d/consul.json
sudo cp "$cfg/consul/consul_${cloud}.service" /etc/systemd/system/consul.service
sudo systemctl enable consul.service
sudo systemctl start consul.service
sleep 10

# ------------------------
# Vault

sudo cp "$cfg/vault/vault.hcl" /etc/vault.d/vault.hcl
sudo cp "$cfg/vault/vault.service" /etc/systemd/system/vault.service
sudo systemctl enable vault.service
sudo systemctl start vault.service

# ------------------------
# Hadoop/Spark

hadoop_version=hadoop-2.7.6

# Hadoop config file to enable HDFS CLI
sudo cp "$cfg/spark/core-site.xml" "/usr/local/$hadoop_version/etc/hadoop"

# Move examples directory to $HOME
sudo mv /opt/shared/examples "$home_dir"
sudo chown -R "$home_dir:$home_dir" "$home_dir/examples"
sudo chmod -R 775 "$home_dir/examples"

# ------------------------
# Nomad

# download
aws s3 cp "s3://nomad-team-test-binary/builds-oss/${nomad_sha}.tar.gz" nomad.tar.gz

# unpack and install
sudo tar -zxvf nomad.tar.gz -C /usr/local/bin/
sudo chmod 0755 /usr/local/bin/nomad
sudo chown root:root /usr/local/bin/nomad

# install config file
sed "s/3 # SERVER_COUNT/${server_count}/g" \
    "/opt/shared/nomad/server.hcl" > /etc/nomad.d/nomad.hcl

# enable as a systemd service
sudo cp /opt/shared/nomad.service /etc/systemd/system/nomad.service
sudo systemctl enable nomad.service
sudo systemctl start nomad.service

# ------------------------
# environment

# Set env vars for tool CLIs
echo "export CONSUL_RPC_ADDR=$ip_address:8400" | sudo tee --append "$home_dir/.bashrc"
echo "export CONSUL_HTTP_ADDR=$ip_address:8500" | sudo tee --append "$home_dir/.bashrc"
echo "export VAULT_ADDR=http://$ip_address:8200" | sudo tee --append "$home_dir/.bashrc"
echo "export NOMAD_ADDR=http://$ip_address:4646" | sudo tee --append "$home_dir/.bashrc"
echo "export JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64/jre"  | sudo tee --append "$home_dir/.bashrc"

# Update PATH
echo "export PATH=$PATH:/usr/local/bin/spark/bin:/usr/local/$hadoop_version/bin" | sudo tee --append "$home_dir/.bashrc"
