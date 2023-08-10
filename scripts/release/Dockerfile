# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Dockerfile for building nomad binaries
# that mimics Vagrant environment as far as required
# for building the scripts and running provision scripts

FROM ubuntu:18.04

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
COPY ./scripts/linux-priv-config.sh /tmp/scripts/linux-priv-config.sh
RUN /tmp/scripts/linux-priv-config.sh

COPY ./scripts/linux-priv-go.sh /tmp/scripts/linux-priv-go.sh
RUN /tmp/scripts/linux-priv-go.sh

COPY ./scripts/vagrant-linux-priv-buf.sh /tmp/scripts/vagrant-linux-priv-buf.sh
RUN /tmp/scripts/vagrant-linux-priv-buf.sh

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
    && mkdir -p /home/vagrant/bin /home/vagrant/.ssh \
    && git config --global user.email "nomad@hashicorp.com" \
    && git config --global user.name "Nomad Release Bot"

## Prepare vagrant user for private github.com deps fetching
RUN git config --global url."ssh://git@github.com".insteadOf "https://github.com" || true
RUN echo 'github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==' > /home/vagrant/.ssh/known_hosts

ARG SSH_KEY
RUN echo "$SSH_KEY" > /home/vagrant/.ssh/id_rsa \
    && chmod 600 /home/vagrant/.ssh/id_rsa
