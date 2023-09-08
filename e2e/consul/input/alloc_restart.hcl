# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "alloc-restart" {
  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    network {
      port "http" {
        to = 8080
      }
    }

    service {
      name = "alloc-restart-http"
      port = "http"
    }

    task "python" {
      driver = "raw_exec"

      config {
        command = "python3"
        args    = ["-m", "http.server", "8080", "--directory", "/tmp"]
      }

      resources {
        cpu    = 16
        memory = 32
      }
    }
  }
}
