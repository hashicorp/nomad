data_dir = "/opt/nomad/data"
bind_addr = "0.0.0.0"

# Enable the server
server {
  enabled = true
  bootstrap_expect = SERVER_COUNT
}

consul {
  address = "127.0.0.1:8500"
}

vault {
  enabled = true
  address = "VAULT_ADDR"
  token = "VAULT_TOKEN"
}

