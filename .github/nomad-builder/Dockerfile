# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

FROM ubuntu:bionic

ENV DEBIAN_FRONTEND=noninteractive

ARG ARCH
ARG GO_VERSION

RUN apt-get update && apt-get install -y --no-install-recommends \
 bash \
 build-essential \
 ca-certificates \
 curl \
 git \
 xz-utils \
 zip

# Get Go and get Going ;)
RUN curl -L https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz | tar -C /opt -zxv

# Best to map the internal user to host user
# ARG DOCKER_USER=default_user
# RUN addgroup $DOCKER_USER && useradd -m -g $DOCKER_USER $DOCKER_USER
# USER $DOCKER_USER
# ENV PATH="/home/${DOCKER_USER}/go/bin:/opt/go/bin:$PATH"

# FIXME: should be owned by the $DOCKER_USER but for whatever reason we get
# permission errors on the host system if we do that, even though it's the same s
ENV PATH="/root/go/bin:/opt/go/bin:$PATH"

RUN git config --global --add safe.directory /build

WORKDIR /build
