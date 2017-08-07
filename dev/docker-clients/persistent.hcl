log_level = "DEBUG"
data_dir = "/tmp/nomad-server"

server {
  enabled = true

  # Self-elect, should be 3 or 5 for production
  bootstrap_expect = 1
}

client {
  enabled = true
  options {
    "docker.privileged.enabled" = "true"
  }
}
