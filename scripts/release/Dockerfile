# Dockerfile for building nomad binaries
# that mimics Vagrant environment as far as required
# for building the scripts and running provision scripts

FROM ubuntu:16.04

RUN apt-get update; apt-get install -y \
            apt-transport-https \
            ca-certificates \
            curl \
            git \
            sudo \
            tree \
            unzip \
            wget

RUN useradd --create-home vagrant \
    && echo 'vagrant      ALL = (ALL) NOPASSWD: ALL' >> /etc/sudoers

# install priv packages
COPY ./scripts/vagrant-linux-priv-config.sh /tmp/scripts/vagrant-linux-priv-config.sh
RUN /tmp/scripts/vagrant-linux-priv-config.sh

COPY ./scripts/vagrant-linux-priv-go.sh /tmp/scripts/vagrant-linux-priv-go.sh
RUN /tmp/scripts/vagrant-linux-priv-go.sh

COPY ./scripts/vagrant-linux-priv-protoc.sh /tmp/scripts/vagrant-linux-priv-protoc.sh
RUN /tmp/scripts/vagrant-linux-priv-protoc.sh

USER vagrant

COPY ./scripts/vagrant-linux-unpriv-ui.sh /tmp/scripts/vagrant-linux-unpriv-ui.sh
RUN /tmp/scripts/vagrant-linux-unpriv-ui.sh
# avoid requiring loading nvm.sh by using a well defined path as an alias to the node version
RUN /bin/bash -c '. ~/.nvm/nvm.sh && ln -s ~/.nvm/versions/node/$(nvm current) ~/.nvm/versions/node/.default'

COPY ./scripts/release/docker-build-all /tmp/scripts/docker-build-all

# Update PATH with GO bin, yarn, and node
ENV GOPATH="/opt/gopath" \
    PATH="/home/vagrant/.nvm/versions/node/.default/bin:/home/vagrant/bin:/opt/gopath/bin:/home/vagrant/.yarn/bin:/home/vagrant/.config/yarn/global/node_modules/.bin:$PATH"

RUN mkdir -p /opt/gopath/src/github.com/hashicorp/nomad
RUN mkdir -p /home/vagrant/bin \
    && git config --global user.email "nomad@hashicorp.com" \
    && git config --global user.name "Nomad Release Bot"
