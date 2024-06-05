# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This is a variation of countdash that uses exec2 for running the envoy
# proxies manually.

job "countdash" {
  group "api" {
    network {
      mode = "bridge"
    }

    service {
      name = "count-api"
      port = "9001"

      connect {
        sidecar_service {}
        sidecar_task {
          driver = "exec2"
          user   = "nobody"
          config {
            command = "/opt/bin/envoy"
            args = [
              "-c",
              "${NOMAD_SECRETS_DIR}/envoy_bootstrap.json",
              "-l",
              "${meta.connect.log_level}",
              "--concurrency",
              "${meta.connect.proxy_concurrency}",
              "--disable-hot-restart"
            ]
            # TODO(shoenig) should not need NOMAD_ values once
            # https://github.com/hashicorp/nomad-driver-exec2/issues/29 is
            # fixed.
            unveil = ["rx:/opt/bin", "rwc:/dev/shm", "r:${NOMAD_TASK_DIR}", "r:${NOMAD_SECRETS_DIR}"]
          }

          resources {
            cpu    = 1000
            memory = 256
          }
        }
      }
    }

    task "backend" {
      driver = "docker"

      config {
        image = "docker.io/hashicorpdev/counter-api:v3"
      }
    }
  }

  group "dashboard" {
    network {
      mode = "bridge"

      port "http" {
        static = 9002
        to     = 9002
      }
    }

    service {
      name = "count-dashboard"
      port = "http"

      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "count-api"
              local_bind_port  = 8080
            }
          }
        }
        sidecar_task {
          driver = "exec2"
          user   = "nobody"
          config {
            command = "/opt/bin/envoy"
            args = [
              "-c",
              "${NOMAD_SECRETS_DIR}/envoy_bootstrap.json",
              "-l",
              "${meta.connect.log_level}",
              "--concurrency",
              "${meta.connect.proxy_concurrency}",
              "--disable-hot-restart"
            ]
            # TODO(shoenig) should not need NOMAD_ values once
            # https://github.com/hashicorp/nomad-driver-exec2/issues/29 is
            # fixed.
            unveil = ["rx:/opt/bin", "rwc:/dev/shm", "r:${NOMAD_TASK_DIR}", "r:${NOMAD_SECRETS_DIR}"]
          }

          resources {
            cpu    = 1000
            memory = 256
          }
        }
      }
    }

    task "dashboard" {
      driver = "docker"

      env {
        COUNTING_SERVICE_URL = "http://${NOMAD_UPSTREAM_ADDR_count_api}"
      }

      config {
        image = "docker.io/hashicorpdev/counter-dashboard:v3"
      }
    }
  }
}
