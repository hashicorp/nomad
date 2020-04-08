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
COPY ./scripts/vagrant-linux-priv-config.sh /tmp/build-tmp/scripts/vagrant-linux-priv-config.sh
RUN /tmp/build-tmp/scripts/vagrant-linux-priv-config.sh

COPY ./scripts/vagrant-linux-priv-go.sh /tmp/build-tmp/scripts/vagrant-linux-priv-go.sh
RUN /tmp/build-tmp/scripts/vagrant-linux-priv-go.sh

COPY ./scripts/vagrant-linux-priv-protoc.sh /tmp/build-tmp/scripts/vagrant-linux-priv-protoc.sh
RUN /tmp/build-tmp/scripts/vagrant-linux-priv-protoc.sh

USER vagrant

COPY ./scripts/vagrant-linux-unpriv-ui.sh /tmp/build-tmp/scripts/vagrant-linux-unpriv-ui.sh
RUN /tmp/build-tmp/scripts/vagrant-linux-unpriv-ui.sh

COPY ./GNUmakefile /tmp/build-tmp/makefile/GNUmakefile
RUN make -C /tmp/build-tmp/makefile deps

COPY ./scripts/vagrant-linux-unpriv-osxcross.sh /tmp/build-tmp/scripts/vagrant-linux-unpriv-osxcross.sh
RUN /tmp/build-tmp/scripts/vagrant-linux-unpriv-osxcross.sh /home/vagrant/osxcross


COPY ./scripts/release/docker-build-all /tmp/scripts/docker-build-all

# Update PATH with GO bin, yarn, and node
ENV NODE_VERSION=v10.15.3
ENV GOPATH="/opt/gopath" \
    PATH="/home/vagrant/.nvm/versions/node/${NODE_VERSION}/bin:/home/vagrant/osxcross/target/bin:/home/vagrant/bin:/opt/gopath/bin:/home/vagrant/.yarn/bin:/home/vagrant/.config/yarn/global/node_modules/.bin:$PATH"

RUN mkdir -p /opt/gopath/src/github.com/hashicorp/nomad
RUN mkdir -p /home/vagrant/bin \
    && git config --global user.email "nomad@hashicorp.com" \
    && git config --global user.name "Nomad Release Bot"
