# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

advertise_addr   = "IP_ADDRESS"
bind_addr        = "0.0.0.0"
client_addr      = "0.0.0.0"
bootstrap_expect = SERVER_COUNT
data_dir         = "/opt/consul/data"
log_level        = "INFO"
retry_join       = ["RETRY_JOIN"]
server           = true
ports = {
  grpc = 8502
}
ui_config {
  enabled = true
}
connect {
  enabled = true
}
