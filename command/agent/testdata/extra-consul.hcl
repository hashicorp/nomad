# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# this unnamed (default) config should merge cleanly onto the basic config
consul {
  address               = "127.0.0.1:9501"
  allow_unauthenticated = false
  token                 = "abracadabra"
  timeout               = "20s"
}

# these alternate configs should be added as an extra consul configs
consul {
  name                   = "alternate"
  server_service_name    = "nomad"
  server_http_check_name = "nomad-server-http-health-check"
  server_serf_check_name = "nomad-server-serf-health-check"
  server_rpc_check_name  = "nomad-server-rpc-health-check"
  client_service_name    = "nomad-client"
  client_http_check_name = "nomad-client-http-health-check"
  address                = "[0:0::1F]:8501"
  allow_unauthenticated  = true
  token                  = "xyzzy"
  auth                   = "username:pass"
}

consul {
  name = "other"

  service_identity {
    aud = ["consul-other.io"]
    ttl = "3h"
  }

  task_identity {
    aud = ["consul-other.io"]
    ttl = "5h"
  }
}
