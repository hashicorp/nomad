data_dir = "/opt/nomad/data"
bind_addr = "0.0.0.0"

# Enable the server
server {
  enabled = true
  bootstrap_expect = SERVER_COUNT
}

name = "nomad@IP_ADDRESS"

consul {
  address = "127.0.0.1:8500"
}

vault {
  enabled = false
  address = "vault.service.consul"
  task_token_ttl = "1h"
  create_from_role = "nomad-cluster"
  token = ""
}

