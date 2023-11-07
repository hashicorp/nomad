# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "nomadagent" {
  type = "system"

  group "linux" {

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

    network {
      mode = "bridge"
      port "api" {
        to = 3000
      }
    }

    service {
      provider = "nomad"
      name     = "holepunch"
      port     = "api"
      tags     = ["monitor"]
      check {
        type     = "http"
        path     = "/health"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "task" {
      driver = "podman"
      user   = "nobody"

      config {
        image = "ghcr.io/shoenig/nomad-holepunch:v0.1.3"
      }

      env {
        HOLEPUNCH_BIND = "0.0.0.0"
        HOLEPUNCH_PORT = "3000"
      }

      identity {
        env = true
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}

