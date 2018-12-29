#!/usr/bin/env bash

set -o errexit

#enable ipv6
echo '{"ipv6":true, "fixed-cidr-v6":"2001:db8:1::/64"}' | sudo tee /etc/docker/daemon.json
sudo service docker restart

apt-get update
apt-get install -y liblxc1 lxc-dev lxc shellcheck
apt-get install -y qemu
bash ./scripts/travis-rkt.sh
bash ./scripts/travis-consul.sh
bash ./scripts/travis-vault.sh
