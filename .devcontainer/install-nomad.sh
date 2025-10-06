# https://developer.hashicorp.com/nomad/tutorials/get-started/gs-install
#!/bin/bash

apt-get update && \
apt-get install -y lsb-release wget gpg coreutils

wget -O- https://apt.releases.hashicorp.com/gpg | gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg

echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/hashicorp.list

apt-get update && apt-get install -y nomad