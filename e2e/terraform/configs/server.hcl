enable_debug = true

log_level = "debug"

data_dir = "/opt/nomad/data"

bind_addr = "0.0.0.0"

# Enable the server
server {
  enabled          = true
  bootstrap_expect = 3
}

consul {
  address = "127.0.0.1:8500"
}

vault {
  enabled          = false
  address          = "http://active.vault.service.consul:8200"
  task_token_ttl   = "1h"
  create_from_role = "nomad-cluster"
  token            = ""
}

telemetry {
  collection_interval        = "1s"
  disable_hostname           = true
  prometheus_metrics         = true
  publish_allocation_metrics = true
  publish_node_metrics       = true
}
