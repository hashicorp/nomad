# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "linux" {
  datacenters = ["dc1"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "limits" {

    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }


    task "zip_bomb" {
      artifact {
        source      = "https://github.com/hashicorp/go-getter/blob/v1.7.0/testdata/decompress-zip/bomb.zip"
        destination = "local/"
      }

      driver = "raw_exec"
      config {
        command = "/usr/bin/false"
      }

      resources {
        cpu    = 16
        memory = 32
      }
    }
  }
}
