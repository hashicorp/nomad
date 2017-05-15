data_dir = "/opt/nomad/data"
bind_addr = "IP_ADDRESS"

# Enable the server
server {
  enabled = true
  bootstrap_expect = SERVER_COUNT
}

name = "nomad@IP_ADDRESS"

consul {
  address = "IP_ADDRESS:8500"
}

vault {
  enabled = false
  address = "http://IP_ADDRESS:8200"
  task_token_ttl = "1h"
  create_from_role = "nomad-cluster"
  token = ""
}

