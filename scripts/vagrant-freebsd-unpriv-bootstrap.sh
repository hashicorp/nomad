#!/bin/sh
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


export GOPATH=/opt/gopath

PATH=$GOPATH/bin:$PATH
export PATH

cd /opt/gopath/src/github.com/hashicorp/nomad && gmake bootstrap
