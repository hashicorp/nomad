FROM ubuntu:17.10

RUN apt-get update -y

RUN apt-get install -y \
    build-essential \
    git \
    golang \
    liblxc1 \
    libpcre3-dev \
    lxc-dev \
    lxc-templates \
    pkg-config

ENV NOMAD_VERSION 0.7.0-beta1
ENV NOMAD_SHA256 174794d96d2617252875e2e2ff9e496120acc4a97be54965c324b9a5d11b37ab

ENV GOPATH=$HOME/gopkg
ENV PATH=$PATH:$GOPATH/bin:/usr/local/lib
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/lib

COPY nomad /bin/nomad

RUN mkdir -p /nomad/data && \
    mkdir -p /etc/nomad && \
    mkdir -p gopkg/src/migrations

RUN go get github.com/stretchr/testify/assert
