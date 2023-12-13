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
        static = 9000
      }
    }

    task "one" {
      driver = "raw_exec"

      env {
        version = "1"
      }

      config {
        image   = "busybox:1"
        command = "nc"

        # change args to update the job, the only changes
        args  = ["-ll", "-p", "1234", "-e", "/bin/cat"]
        ports = ["db"]
      }

      resources {
        cpu    = 20
        memory = 20
      }
    }
  }
}
