# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "cpustress" {
  # make sure every node has nonzero cpu usage metrics
  type = "system"

  group "cpustress" {
    constraint {
      attribute = "${attr.kernel.name}"
      value     = "linux"
    }

    update {
      min_healthy_time = "4s"
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    service {
      provider = "nomad"
      name     = "stress"
      tags     = ["cpu"]
    }

    task "cpustress" {
      driver = "pledge"

      config {
        command  = "stress"
        args     = ["--cpu", "1", ]
        promises = "stdio rpath proc"
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}

