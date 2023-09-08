# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Increase log verbosity
log_level = "DEBUG"

region = "foo"

# Setup data dir
data_dir = "/tmp/server1"

# Enable the server
server {
  enabled = true

  # Self-elect, should be 3 or 5 for production
  bootstrap_expect = 1
}

tls {
  http = true
  rpc  = true

  ca_file   = "certs/nomad-agent-ca.pem"
  cert_file = "certs/foo-server-nomad.pem"
  key_file  = "certs/foo-server-nomad-key.pem"

  verify_server_hostname = true
  verify_https_client    = true
}
