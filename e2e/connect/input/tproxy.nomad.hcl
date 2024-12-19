# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "countdash" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "api" {
    network {
      mode = "bridge"
    }

    service {
      name = "count-api"
      port = "9001"

      check {
        type     = "http"
        path     = "/health"
        expose   = true
        interval = "3s"
        timeout  = "1s"

        check_restart {
          limit = 0
        }
      }

      connect {
        sidecar_service {
          proxy {
            transparent_proxy {}
          }
        }
      }
    }

    task "web" {
      driver = "docker"

      config {
        image          = "hashicorpdev/counter-api:v3"
        auth_soft_fail = true
      }
    }
  }

  group "dashboard" {
    network {
      mode = "bridge"

      port "http" {
        static = 9010
        to     = 9002
      }
    }

    service {
      name = "count-dashboard"
      port = "9002"

      check {
        type     = "http"
        path     = "/health"
        expose   = true
        interval = "3s"
        timeout  = "1s"

        check_restart {
          limit = 0
        }
      }

      connect {
        sidecar_service {
          proxy {
            transparent_proxy {}
          }
        }
      }
    }

    task "dashboard" {
      driver = "docker"

      env {
        COUNTING_SERVICE_URL = "http://count-api.virtual.consul"
      }

      config {
        image          = "hashicorpdev/counter-dashboard:v3"
        auth_soft_fail = true
      }
    }
  }
}
