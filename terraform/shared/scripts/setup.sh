#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


#set -e

# Disable interactive apt-get prompts
export DEBIAN_FRONTEND=noninteractive

cd /ops

CONFIGDIR=/ops/shared/config
sudo apt-get install -yq  apt-utils

# Install HashiCorp products
CONSULVERSION=1.17.0
VAULTVERSION=1.15.4
NOMADVERSION=1.7.1
CONSULTEMPLATEVERSION=0.35.0

sudo apt-get update && sudo apt-get install gpg
wget -O- https://apt.releases.hashicorp.com/gpg | sudo gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/hashicorp.list
sudo apt-get update
sudo apt-get install -yq consul="${CONSULVERSION}*" \
                    vault="${VAULTVERSION}*" \
                    nomad="${NOMADVERSION}*" \
                    consul-template="${CONSULTEMPLATEVERSION}*"

# Dependencies
sudo apt-get install -yq software-properties-common
sudo apt-get update
sudo apt-get install -yq unzip tree redis jq curl tmux openjdk-8-jdk

# Disable the firewall
sudo ufw disable || echo "ufw not installed"

# Docker
distro=$(lsb_release -si | tr '[:upper:]' '[:lower:]')
sudo apt-get install -yq apt-transport-https ca-certificates gnupg2 
# Add Docker's official GPG key:
sudo apt-get update
sudo apt-get install ca-certificates curl gnupg
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

# Add the repository to apt-get sources:
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
sudo apt-get install -yq docker-ce docker-ce-cli containerd.io docker-buildx-plugin


# # Needs testing, updating and fixing
if [[ ! -z ${INSTALL_NVIDIA_DOCKER+x} ]]; then 

  # Install official NVIDIA driver package
  ### Needs updating for 2204

  # Install nvidia container support
  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg \
    && curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
      sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
      sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list

  sudo apt-get update
  sudo apt-get install -y nvidia-container-toolkit

  sudo nvidia-ctk runtime configure --runtime=docker
  sudo systemctl restart docker

#   # Install official NVIDIA driver package
#   # This is why we added gnupg-curl, otherwise, the following fails with "gpgkeys: protocol `https' not supported"
#   sudo apt-key adv --fetch-keys https://developer.download.nvidia.com/compute/cuda/repos/ubuntu1604/x86_64/3bf863cc.pub
#   sudo sh -c 'echo "deb http://developer.download.nvidia.com/compute/cuda/repos/ubuntu1604/x86_64 /" > /etc/apt/sources.list.d/cuda.list'
#   sudo apt-get update && sudo apt-get install -yq --no-install-recommends --allow-unauthenticated linux-headers-generic dkms cuda-drivers

#   # Install nvidia-docker and nvidia-docker-plugin
#   # from: https://github.com/NVIDIA/nvidia-docker#ubuntu-140416041804-debian-jessiestretch
#   wget -P /tmp https://github.com/NVIDIA/nvidia-docker/releases/download/v1.0.1/nvidia-docker_1.0.1-1_amd64.deb
#   sudo dpkg -i /tmp/nvidia-docker*.deb && rm /tmp/nvidia-docker*.deb
#   curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
#   distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
#   curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | \
#     sudo tee /etc/apt/sources.list.d/nvidia-docker.list

#   sudo apt-get update
#   sudo apt-get install -yq --allow-unauthenticated nvidia-docker2
# fi

