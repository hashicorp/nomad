# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

data_dir  = "/opt/nomad/data"
bind_addr = "0.0.0.0"

# Enable the client
client {
  enabled = true
  options {
    "driver.raw_exec.enable"    = "1"
    "docker.privileged.enabled" = "true"
  }
}

consul {
  address = "127.0.0.1:8500"
}

vault {
  enabled = true
  address = "http://active.vault.service.consul:8200"
}
