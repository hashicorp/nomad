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

# Add Docker bridge network IP to /etc/resolv.conf (at the top)
DOCKER_BRIDGE_IP_ADDRESS=$(/usr/local/bin/sockaddr eval 'GetInterfaceIP "docker0"')
echo "nameserver $DOCKER_BRIDGE_IP_ADDRESS" | sudo tee /etc/resolv.conf.new
cat /etc/resolv.conf | sudo tee --append /etc/resolv.conf.new
sudo mv /etc/resolv.conf.new /etc/resolv.conf

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
Documentation=man:podman-varlink(1)

[Socket]
ListenStream=%t/podman/io.podman
SocketMode=0600

[Install]
WantedBy=sockets.targett
EOF

# enable as a systemd service
sudo cp "$NOMAD_SRC/nomad.service" /etc/systemd/system/nomad.service

sudo systemctl enable nomad.service
sudo systemctl daemon-reload
sudo systemctl start io.podman
sudo systemctl restart nomad.service
