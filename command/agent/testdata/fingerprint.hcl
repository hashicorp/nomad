// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

client {
  enabled = true

  fingerprint "env_aws" {
    retry_interval  = "5m"
    retry_attempts  = 3
    exit_on_failure = true
  }

  fingerprint "env_azure" {
    retry_interval  = "10m"
    retry_attempts  = 5
    exit_on_failure = false
  }

  fingerprint "env_gce" {
    retry_interval = "2m"
    retry_attempts = -1
  }

  fingerprint "env_digitalocean" {
    retry_interval = "1m"
  }
}
