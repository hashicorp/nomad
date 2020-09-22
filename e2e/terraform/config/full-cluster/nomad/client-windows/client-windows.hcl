enable_debug = true

log_level = "debug"

log_file = "C:\\opt\\nomad\\nomad.log"

data_dir = "C:\\opt\\nomad\\data"

bind_addr = "0.0.0.0"

# Enable the client
client {
  enabled = true

  options {
    # Allow rawexec jobs
    "driver.raw_exec.enable" = "1"
  }
}

consul {
  address = "127.0.0.1:8500"
}

vault {
  enabled = true
  address = "http://active.vault.service.consul:8200"
}

telemetry {
  collection_interval        = "1s"
  disable_hostname           = true
  prometheus_metrics         = true
  publish_allocation_metrics = true
  publish_node_metrics       = true
}
