# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

plugin_dir = "/opt/nomad/plugins"

client {
  enabled = true
  options = {
    "user.denylist" = "www-data"
  }
}

plugin "nomad-driver-podman" {
  config {
    volumes {
      enabled = true
    }
    auth {
      helper = "test.sh"
      config = "/etc/auth.json"
    }
  }
}

plugin "nomad-driver-ecs" {
  config {
    enabled = true
    cluster = "nomad-rtd-e2e"
    region  = "us-east-1"
  }
}

plugin "raw_exec" {
  config {
    enabled = true
  }
}

plugin "docker" {
  config {
    allow_privileged = true

    volumes {
      enabled = true
    }
  }
}

plugin "nomad-pledge-driver" {
  config {
    pledge_executable = "/usr/local/bin/pledge"
  }
}
