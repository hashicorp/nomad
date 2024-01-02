# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "cn-demo" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "generator" {
    network {
      port "api" {}
    }

    service {
      name = "uuid-api"
      port = "${NOMAD_PORT_api}"
      task = "generate"

      connect {
        native = true
      }
    }

    task "generate" {
      driver = "docker"

      config {
        image        = "hashicorpdev/uuid-api:v5"
        network_mode = "host"
      }

      env {
        BIND = "0.0.0.0"
        PORT = "${NOMAD_PORT_api}"
      }
    }
  }

  group "frontend" {
    network {
      port "http" {
        static = 9800
      }
    }

    service {
      name = "uuid-fe"
      port = "9800"
      task = "frontend"

      connect {
        native = true
      }
    }

    task "frontend" {
      driver = "docker"

      config {
        image        = "hashicorpdev/uuid-fe:v5"
        network_mode = "host"
      }

      env {
        UPSTREAM = "uuid-api"
        BIND     = "0.0.0.0"
        PORT     = "9800"
      }
    }
  }
}
