#!/bin/bash
set -uex
export PATH=$PATH:/usr/local/go/bin/:$(/usr/local/go/bin/go env GOPATH)/bin
sudo sysctl -w vm.max_map_count=655300
sudo service nomad stop
make clean
make bootstrap
make prerelease
make release
sudo rm /usr/local/bin/nomad
sudo cp pkg/linux_amd64/nomad /usr/local/bin/nomad
sudo service nomad start