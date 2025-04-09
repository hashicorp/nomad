# syntax=docker/dockerfile:1

FROM ubuntu:bionic

ENV DEBIAN_FRONTEND=noninteractive

ARG ARCH="amd64"
ARG GO_VERSION="1.24.1"
ARG NODE_VERSION="20.9.0"

RUN apt-get update && apt-get install -y --no-install-recommends \
 bash \
 build-essential \
 ca-certificates \
 curl \
 # node 20 requires gcc 8.3+
 g++-8 \
 gcc-8 \
 git \
 python3 \
 xz-utils

RUN update-alternatives --install /usr/bin/python python /usr/bin/python3 1
RUN update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-8 1
RUN update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-8 1

# Get Go and get Going ;)
RUN curl -L https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz | tar -C /opt -zxv
ENV PATH="/opt/go/bin:$PATH"

# Build node from source (node 20 and above binaries require newer glibc)
RUN curl -fsSL https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION.tar.xz | tar -xJ && \
    cd node-v$NODE_VERSION && \
    ./configure --prefix=/opt/node && \
    make -j$(($(nproc) / 2)) && \
    make install && \
    cd .. && rm -rf node-v$NODE_VERSION

ENV PATH="/opt/node/bin:$PATH"

# Install yarn
RUN npm install -g yarn

ENTRYPOINT /bin/bash