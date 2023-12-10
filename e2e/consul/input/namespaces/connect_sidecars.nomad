# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "connect_sidecars" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "api" {

    consul {
      namespace = "apple"
    }

    network {
      mode = "bridge"
    }

    service {
      name = "count-api"
      port = "9001"

      connect {
        sidecar_service {}
      }

      check {
        expose   = true
        name     = "api-health"
        type     = "http"
        path     = "/health"
        interval = "5s"
        timeout  = "3s"
      }
    }

    task "web" {
      driver = "docker"

      config {
        image = "hashicorpdev/counter-api:v3"
      }
    }
  }

  group "api-z" {

    # consul namespace not set

    network {
      mode = "bridge"
    }

    service {
      name = "count-api-z"
      port = "9001"

      connect {
        sidecar_service {}
      }

      check {
        expose   = true
        name     = "api-health"
        type     = "http"
        path     = "/health"
        interval = "5s"
        timeout  = "3s"
      }
    }

    task "web-z" {
      driver = "docker"

      config {
        image = "hashicorpdev/counter-api:v3"
      }
    }
  }

  group "dashboard" {

    consul {
      namespace = "apple"
    }

    network {
      mode = "bridge"

      port "http" {
        static = 9002
        to     = 9002
      }
    }

    service {
      name = "count-dashboard"
      port = "9002"

      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "count-api"
              local_bind_port  = 8080
            }
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
        image = "hashicorpdev/counter-dashboard:v3"
      }
    }
  }

  group "dashboard-z" {

    # consul namespace not set

    network {
      mode = "bridge"

      port "http" {
        static = 9003
        to     = 9002
      }
    }

    service {
      name = "count-dashboard-z"
      port = "9003"

      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "count-api-z"
              local_bind_port  = 8080
            }
          }
        }
      }
    }

    task "dashboard" {
      driver = "docker"

      env {
        COUNTING_SERVICE_URL = "http://${NOMAD_UPSTREAM_ADDR_count_api-z}"
      }

      config {
        image = "hashicorpdev/counter-dashboard:v3"
      }
    }
  }
}
