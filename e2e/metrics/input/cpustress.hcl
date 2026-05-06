# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

job "cpustress" {

  # make sure every node has nonzero cpu usage metrics
  type = "system"

  group "cpustress" {
    constraint {
      attribute = "${attr.kernel.name}"
      value     = "linux"
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    update {
      min_healthy_time = "4s"
    }

    service {
      provider = "nomad"
      name     = "stress"
      tags     = ["cpu"]
    }

    task "cpustress" {
      driver = "exec2"

      config {
        command = "stress"
        args    = ["--cpu", "1"]
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
