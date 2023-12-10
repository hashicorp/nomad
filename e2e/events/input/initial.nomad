# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "deployment_auto.nomad" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "one" {
    count = 3

    update {
      max_parallel     = 3
      auto_promote     = true
      canary           = 2
      min_healthy_time = "1s"
    }

    network {
      port "db" {
        to = 1234
      }
    }

    task "one" {
      driver = "docker"

      env {
        version = "0"
      }

      config {
        image   = "busybox:1"
        command = "nc"
        args    = ["-ll", "-p", "1234", "-e", "/bin/cat"]
        ports   = ["db"]
      }

      resources {
        cpu    = 20
        memory = 20
      }
    }
  }
}
