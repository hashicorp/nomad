# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "foo" {
  datacenters = ["dc1"]

  group "bar" {
    count          = 3
    shutdown_delay = "14s"

    network {
      mode = "bridge"

      port "http" {
        static       = 80
        to           = 8080
        host_network = "public"
      }

      dns {
        servers = ["8.8.8.8"]
        options = ["ndots:2", "edns0"]
      }
    }

    service {
      name        = "connect-service"
      tags        = ["foo", "bar"]
      canary_tags = ["canary", "bam"]
      port        = "1234"

      connect {
        sidecar_service {
          tags = ["side1", "side2"]

          proxy {
            local_service_port = 8080

            upstreams {
              destination_name       = "other-service"
              destination_peer       = "10.0.0.1:6379"
              destination_type       = "tcp"
              local_bind_port        = 4567
              local_bind_address     = "0.0.0.0"
              local_bind_socket_path = "/var/run/testsocket.sock"
              local_bind_socket_mode = "0666"
              datacenter             = "dc1"

              mesh_gateway {
                mode = "local"
              }
            }
          }
        }

        sidecar_task {
          resources {
            cpu    = 500
            memory = 1024
          }

          env {
            FOO = "abc"
          }

          shutdown_delay = "5s"
        }
      }
    }

    task "bar" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "echo hi"]
      }

      resources {
        network {
          mbits = 10
        }
      }
    }
  }
}
