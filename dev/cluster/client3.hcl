# Increase log verbosity
log_level = "DEBUG"

# Setup data dir
data_dir = "/tmp/client3"

# Give the agent a unique name. Defaults to hostname
name = "client3"

# Enable the client
client {
  enabled = true
  meta {
    "rack" = "r1"
  }
  server_join {
    retry_join = ["127.0.0.1:4647", "127.0.0.1:5647", "127.0.0.1:6647"]
  }
}

ports {
  http = 8646
}
