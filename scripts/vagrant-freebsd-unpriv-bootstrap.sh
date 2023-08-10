#!/bin/sh
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


export GOPATH=/opt/gopath

PATH=$GOPATH/bin:$PATH
export PATH

cd /opt/gopath/src/github.com/hashicorp/nomad && gmake bootstrap
