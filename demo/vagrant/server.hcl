# Increase log verbosity
log_level = "DEBUG"

# Setup data dir
data_dir = "/tmp/server1"

vault {
  enabled = true
  # address = "127.0.0.1:8200"
  token_role_name = "foobar"
  # periodic_token = "09e54c4d-a9b6-f1b8-fb41-e87a263d4da9"
}

# Enable the server
server {
    enabled = true

    # Self-elect, should be 3 or 5 for production
    bootstrap_expect = 1
}
