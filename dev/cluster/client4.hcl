# Increase log verbosity
log_level = "DEBUG"
datacenter = "dc2"
# Setup data dir
data_dir = "/tmp/client4"

# Give the agent a unique name. Defaults to hostname
name = "client4"

# Enable the client
client {
  enabled = true
  meta {
   "rack" = "r2"
  }
  server_join {
    retry_join = ["127.0.0.1:4647", "127.0.0.1:5647", "127.0.0.1:6647"]
  }
}

ports {
  http = 9646
}
