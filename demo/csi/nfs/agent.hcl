# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

data_dir = "/tmp/nomad/data"

server {
  enabled = true

  bootstrap_expect = 1
}

client {
  enabled = true
  host_volume "host-nfs" {
    path = "/srv/host-nfs"
  }
}

plugin "docker" {
  config {
    allow_privileged = true
  }
}
