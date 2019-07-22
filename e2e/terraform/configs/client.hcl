enable_debug = true

log_level = "DEBUG"

data_dir = "/opt/nomad/data"

bind_addr = "0.0.0.0"

# Enable the client
client {
  enabled = true

  options {
    # Allow jobs to run as root
    "user.blacklist" = ""

    # Allow rawexec jobs
    "driver.raw_exec.enable" = "1"

    # Allow privileged docker jobs
    "docker.privileged.enabled" = "true"
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
