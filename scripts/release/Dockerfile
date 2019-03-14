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
ADD ./scripts/vagrant-linux-priv-config.sh /tmp/scripts/vagrant-linux-priv-config.sh
RUN /tmp/scripts/vagrant-linux-priv-config.sh

ADD ./scripts/vagrant-linux-priv-go.sh /tmp/scripts/vagrant-linux-priv-go.sh
RUN /tmp/scripts/vagrant-linux-priv-go.sh

ADD ./scripts/vagrant-linux-priv-protoc.sh /tmp/scripts/vagrant-linux-priv-protoc.sh
RUN /tmp/scripts/vagrant-linux-priv-protoc.sh

USER vagrant

ADD ./scripts/vagrant-linux-unpriv-ui.sh /tmp/scripts/vagrant-linux-unpriv-ui.sh
RUN /tmp/scripts/vagrant-linux-unpriv-ui.sh

# Update PATH with GO bin, yarn, and node
ENV GOPATH="/opt/gopath" \
    PATH="/home/vagrant/bin:/opt/gopath/bin:/home/vagrant/.yarn/bin:/home/vagrant/.config/yarn/global/node_modules/.bin:$PATH"

RUN mkdir -p /opt/gopath/src/github.com/hashicorp/nomad
RUN mkdir -p /home/vagrant/bin \
    && git config --global user.email "nomad@hashicorp.com" \
    && git config --global user.name "Nomad Release Bot"

COPY --chown=vagrant:vagrant ./scripts/release/docker-build-all /home/vagrant/bin/docker-build-all
