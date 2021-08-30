tls {
  http = true
  rpc  = true

  ca_file   = "/etc/nomad.d/tls/ca.crt"
  cert_file = "/etc/nomad.d/tls/agent.crt"
  key_file  = "/etc/nomad.d/tls/agent.key"

  verify_server_hostname = true
  verify_https_client    = true
}

consul {
  address = "127.0.0.1:8501"
  ssl     = true

  ca_file   = "/etc/nomad.d/tls/ca.crt"
  cert_file = "/etc/nomad.d/tls/agent.crt"
  key_file  = "/etc/nomad.d/tls/agent.key"
}
