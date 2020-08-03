#!/bin/bash
# installs and configures the desired build of Nomad as a client
set -o errexit
set -o nounset

CLOUD="$1"
NOMAD_CONFIG="$2"

# Consul
CONSUL_SRC=/ops/shared/consul
CONSUL_DEST=/etc/consul.d

sudo cp "$CONSUL_SRC/base.json" "$CONSUL_DEST/"
sudo cp "$CONSUL_SRC/retry_$CLOUD.json" "$CONSUL_DEST/"
sudo cp "$CONSUL_SRC/consul_$CLOUD.service" /etc/systemd/system/consul.service

sudo systemctl enable consul.service
sudo systemctl daemon-reload
sudo systemctl restart consul.service
sleep 10

# Add hostname to /etc/hosts
echo "127.0.0.1 $(hostname)" | sudo tee --append /etc/hosts

# Use dnsmasq first and then docker bridge network for DNS resolution
DOCKER_BRIDGE_IP_ADDRESS=$(/usr/local/bin/sockaddr eval 'GetInterfaceIP "docker0"')
cat <<EOF > /tmp/resolv.conf
nameserver 127.0.0.1
nameserver $DOCKER_BRIDGE_IP_ADDRESS
EOF
sudo mv /tmp/resolv.conf /etc/resolv.conf

# need to get the AWS DNS address from the VPC...
# this is pretty hacky but will work for any typical case
MAC=$(curl -s --fail http://169.254.169.254/latest/meta-data/mac)
CIDR_BLOCK=$(curl -s --fail "http://169.254.169.254/latest/meta-data/network/interfaces/macs/$MAC/vpc-ipv4-cidr-block")
VPC_DNS_ROOT=$(echo "$CIDR_BLOCK" | cut -d'.' -f1-3)
echo "nameserver ${VPC_DNS_ROOT}.2" > /tmp/dnsmasq-resolv.conf
sudo mv /tmp/dnsmasq-resolv.conf /var/run/dnsmasq/resolv.conf

sudo systemctl restart dnsmasq
sudo systemctl restart docker

# Nomad

NOMAD_SRC=/ops/shared/nomad
NOMAD_DEST=/etc/nomad.d
NOMAD_CONFIG_FILENAME=$(basename "$NOMAD_CONFIG")

# assert Nomad binary's permissions
if [[ -f /usr/local/bin/nomad ]]; then
    sudo chmod 0755 /usr/local/bin/nomad
    sudo chown root:root /usr/local/bin/nomad
fi

sudo cp "$NOMAD_SRC/base.hcl" "$NOMAD_DEST/"
sudo cp "$NOMAD_SRC/$NOMAD_CONFIG" "$NOMAD_DEST/$NOMAD_CONFIG_FILENAME"

# Setup Host Volumes
sudo mkdir -p /tmp/data

# Install CNI plugins
sudo mkdir -p /opt/cni/bin
wget -q -O - \
     https://github.com/containernetworking/plugins/releases/download/v0.8.6/cni-plugins-linux-amd64-v0.8.6.tgz \
    | sudo tar -C /opt/cni/bin -xz

# enable varlink socket (not included in ubuntu package)
cat > /etc/systemd/system/io.podman.service << EOF
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

cat > /etc/systemd/system/io.podman.socket << EOF
[Unit]
Description=Podman Remote API Socket
Documentation=man:podman-varlink(1) https://podman.io/blogs/2019/01/16/podman-varlink.html

[Socket]
ListenStream=%t/podman/io.podman
SocketMode=0600

[Install]
WantedBy=sockets.target
EOF

# enable as a systemd service
sudo cp "$NOMAD_SRC/nomad.service" /etc/systemd/system/nomad.service

sudo systemctl enable nomad.service
sudo systemctl daemon-reload
sudo systemctl start io.podman
sudo systemctl restart nomad.service
