tls {
  http = true
  rpc  = true

  ca_file   = "ca.pem"
  cert_file = "client.pem"
  key_file  = "client-key.pem"

  verify_server_hostname = true
  verify_https_client    = true
}
