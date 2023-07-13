# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "cpustress" {
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
        args     = ["--cpu", "2", ]
        promises = "stdio rpath proc"
      }

      resources {
        cores  = 2
        memory = 64
      }
    }
  }
}

