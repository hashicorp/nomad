data_dir = "/opt/nomad/data"
bind_addr = "IP_ADDRESS"
name = "nomad@IP_ADDRESS"

# Enable the client
client {
  enabled = true
}

consul {
  address = "127.0.0.1:8500"
}

vault {
  enabled = true
  address = "vault.service.consul"
}
