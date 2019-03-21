#!/usr/bin/env bash

# Install Development utilities
apt-get install -y \
	      curl \
	      default-jre \
	      htop \
	      jq \
	      qemu \
	      silversearcher-ag \
	      tree \
	      unzip \
	      vim


# Set hostname -> IP to make advertisement work as expected
ip=$(ip route get 1 | awk '{print $NF; exit}')
hostname=$(hostname)
sed -i -e "s/.*nomad.*/${ip} ${hostname}/" /etc/hosts

# Ensure we cd into the working directory on login
if ! grep "cd /opt/gopath/src/github.com/hashicorp/nomad" /home/vagrant/.profile ; then
	  echo 'cd /opt/gopath/src/github.com/hashicorp/nomad' >> /home/vagrant/.profile
fi
