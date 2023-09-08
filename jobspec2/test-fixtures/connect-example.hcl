# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "web" {
  datacenters = ["dc1"]
  group "web" {
    network {
      mode = "bridge"

      port "http" {
        static = 80
        to     = 8080
      }
    }

    service {
      name = "website"
      port = "8080"

      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "database"
              local_bind_port  = 5432
              config {
                connect_timeout_ms = 9999
              }
            }
          }
        }
      }
    }

    task "httpserver" {
      driver = "docker"
      env {
        COUNTING_SERVICE_URL = "http://${NOMAD_UPSTREAM_ADDR_database}"
      }
      config {
        image          = "hashicorp/website:v1"
        auth_soft_fail = true
      }
    }
  }
}
