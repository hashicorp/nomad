# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Increase log verbosity
log_level = "DEBUG"

# Setup data dir
data_dir = "/tmp/server1"

# Give the agent a unique name. Defaults to hostname
name = "server1"

# Enable the server
server {
  enabled = true

  server_join {
    retry_join = ["127.0.0.1:5648", "127.0.0.1:6648"]
  }

  # Self-elect, should be 3 or 5 for production
  bootstrap_expect = 3
}
