# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Increase log verbosity
log_level = "DEBUG"

# Setup data dir
data_dir = "/tmp/nomad-dev-cluster/serverstandalone"

# Give the agent a unique name. Defaults to hostname
name = "serverstandalone"

# Enable the server
server {
  enabled = true

  bootstrap_expect = 1
}
