# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "example" {
  vault {
    policies = ["job"]
  }

  group "cache" {
    vault {
      policies = ["group"]
    }

    task "redis" {}

    task "redis2" {
      vault {
        policies     = ["task"]
        env          = false
        disable_file = true
      }
    }
  }

  group "cache2" {
    task "redis" {}
  }
}
