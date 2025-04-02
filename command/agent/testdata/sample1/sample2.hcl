# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

"advertise" = {
  "http" = "host.example.com"
  "rpc"  = "host.example.com"
  "serf" = "host.example.com"
}

"autopilot" = {
  "cleanup_dead_servers" = true
}

"consul" = {
  "client_auto_join" = false
  "server_auto_join" = false
  "token"            = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
}

rpc {
  accept_backlog           = 256
  keep_alive_interval      = "30s"
  connection_write_timeout = "10s"
  max_stream_window_size   = 262144
  stream_open_timeout      = "75s"
  stream_close_timeout     = "5m"
}

vault = {
  enabled = true
}

audit {
  enabled = true

  sink "file" {
    type               = "file"
    format             = "json"
    delivery_guarantee = "enforced"
    path               = "/opt/nomad/audit.log"
    rotate_bytes       = 100
    rotate_duration    = "24h"
    rotate_max_files   = 10
  }

  filter "default" {
    type       = "HTTPEvent"
    endpoints  = ["/v1/metrics"]
    stages     = ["*"]
    operations = ["*"]
  }
}
