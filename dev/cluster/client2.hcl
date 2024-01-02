# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Increase log verbosity
log_level = "DEBUG"

# Setup data dir
data_dir = "/tmp/nomad-dev-cluster/client2"

# Give the agent a unique name. Defaults to hostname
name = "client2"

# Enable debugging
enable_debug = true

# Enable the client
client {
  enabled = true

  server_join {
    retry_join = ["127.0.0.1:4647", "127.0.0.1:5647", "127.0.0.1:6647"]
  }

  meta {
    tag = "foo"
  }
}

plugin "raw_exec" {
  config {
    enabled = true
  }
}

ports {
  http = 8646
}
