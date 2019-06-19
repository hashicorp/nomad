#!/usr/bin/env bash

# Add the Docker repository
apt-key adv \
	      --keyserver hkp://p80.pool.sks-keyservers.net:80 \
	      --recv-keys 9DC858229FC7DD38854AE2D88D81803C0EBFCD88
add-apt-repository \
	  "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
	$(lsb_release -cs) \
	stable"

# Update with i386, Go and Docker
apt-get update

apt-get install -y docker-ce

# Restart Docker in case it got upgraded
systemctl restart docker.service

# Ensure Docker can be used by vagrant user
usermod -aG docker vagrant
