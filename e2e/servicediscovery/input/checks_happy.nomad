# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "checks_happy" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    network {
      mode = "host"
      port "http" {}
    }

    service {
      provider = "nomad"
      name     = "http-server"
      port     = "http"
      check {
        name     = "http-server-check"
        type     = "http"
        path     = "/"
        interval = "2s"
        timeout  = "1s"
      }
    }

    task "python-http" {
      driver = "raw_exec"
      config {
        command = "python3"
        args    = ["-m", "http.server", "${NOMAD_PORT_http}"]
      }
    }
  }
}
