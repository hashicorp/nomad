#!/bin/bash
set -uex
sudo service nomad stop
make clean
make dev
sudo rm /usr/local/bin/nomad
sudo mv pkg/linux_amd64/nomad /usr/local/bin/nomad
sudo service nomad start