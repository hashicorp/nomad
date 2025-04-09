# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

FROM ubuntu:bionic

ENV DEBIAN_FRONTEND=noninteractive

ARG ARCH="amd64"
ARG GO_VERSION="1.24.1"

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
ENV PATH="/root/go/bin:/opt/go/bin:$PATH"

RUN git config --global --add safe.directory /build

WORKDIR /build
