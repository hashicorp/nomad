# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "simpleweb" {
  datacenters = ["dc1"]
  type        = "system"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "simpleweb" {
    network {
      port "http" {
        to = 8080
      }
    }
    task "simpleweb" {
      driver = "docker"

      config {
        image = "nginx:latest"

        ports = ["http"]
      }

      resources {
        cpu    = 256
        memory = 128
      }

      // TODO(tgross): this isn't passing health checks
      service {
        port = "http"
        name = "simpleweb"
        tags = ["simpleweb"]

        check {
          type     = "tcp"
          port     = "http"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
