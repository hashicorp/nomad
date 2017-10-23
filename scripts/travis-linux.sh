#!/usr/bin/env bash

set -o errexit

apt-get update
apt-get install -y liblxc1 lxc-dev lxc shellcheck
apt-get install -y qemu
bash ./scripts/travis-rkt.sh
bash ./scripts/travis-consul.sh
bash ./scripts/travis-vault.sh
