enable_debug = true

log_level = "debug"

data_dir = "/opt/nomad/data"

bind_addr = "0.0.0.0"

consul {
  address = "127.0.0.1:8500"
}

audit {
  enabled = true
}

telemetry {
  collection_interval        = "1s"
  disable_hostname           = true
  prometheus_metrics         = true
  publish_allocation_metrics = true
  publish_node_metrics       = true
}
