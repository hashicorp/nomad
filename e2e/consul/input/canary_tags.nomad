# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "consul_canary_test" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "consul_canary_test" {
    count = 2

    network {
      port "db" {}
    }

    task "consul_canary_test" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "nc"
        args    = ["-ll", "-p", "1234", "-e", "/bin/cat"]

        port_map {
          db = 1234
        }
      }

      resources {
        cpu    = 100
        memory = 100
      }

      service {
        name        = "canarytest"
        tags        = ["foo", "bar"]
        canary_tags = ["foo", "canary"]
      }
    }

    update {
      max_parallel     = 1
      canary           = 1
      min_healthy_time = "1s"
      health_check     = "task_states"
      auto_revert      = false
    }

    restart {
      attempts = 0
      delay    = "0s"
      mode     = "fail"
    }
  }
}
