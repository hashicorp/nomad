#!/bin/bash
# installs and configures the desired build of Nomad as a server
set -o errexit
set -o nounset

nomad_sha=$1

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
