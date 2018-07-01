# Increase log verbosity
log_level = "DEBUG"

# Setup data dir
data_dir = "/data/data/nomad/storage/client1"

# Give the agent a unique name. Defaults to hostname
name = "client1"

# Enable the client
client {
    enabled = true

    memory_total_mb = 1000

    # For demo assume we are talking to server1. For production,
    # this should be like "nomad.service.consul:4647" and a system
    # like Consul used for service discovery.
    servers = ["10.0.0.18:4647"]

  options = {
    "driver.raw_exec.enable" = "1"
  }

}

# Modify our port to avoid a collision with server1
ports {
    http = 5656
}
