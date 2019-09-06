#!/usr/bin/env bash

set -o errexit

#enable ipv6
echo '{"ipv6":true, "fixed-cidr-v6":"2001:db8:1::/64"}' | sudo tee /etc/docker/daemon.json
sudo service docker restart

# Ignore apt-get update errors to avoid failing due to misbehaving repo;
# true errors would fail in the apt-get install phase
apt-get update || true

apt-get install -y qemu shellcheck
bash ./scripts/travis-rkt.sh
bash ./scripts/travis-consul.sh
bash ./scripts/travis-vault.sh
