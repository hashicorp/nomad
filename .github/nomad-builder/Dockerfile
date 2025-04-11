# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

FROM ubuntu:bionic

ENV DEBIAN_FRONTEND=noninteractive

ARG GO_VERSION

RUN apt-get update && apt-get install -y --no-install-recommends \
 bash \
 build-essential \
 ca-certificates \
 crossbuild-essential-arm64 \
 curl \
 gcc-aarch64-linux-gnu \
 git \
 xz-utils \
 zip

# Get Go and get Going ;)
RUN curl -L https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | tar -C /opt -zxv

ENV PATH="/root/go/bin:/opt/go/bin:$PATH"

RUN git config --global --add safe.directory /build

WORKDIR /build
