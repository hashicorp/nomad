tls {
  http = true
  rpc  = true

  ca_file   = "ca.pem"
  cert_file = "dev.pem"
  key_file  = "dev-key.pem"

  verify_server_hostname = true
  verify_https_client    = true
}
