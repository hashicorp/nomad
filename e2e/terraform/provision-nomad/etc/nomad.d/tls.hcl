# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

tls {
  http = true
  rpc  = true

  ca_file   = "/etc/nomad.d/tls/ca.crt"
  cert_file = "/etc/nomad.d/tls/agent.crt"
  key_file  = "/etc/nomad.d/tls/agent.key"

  verify_server_hostname = true
  verify_https_client    = true
}
