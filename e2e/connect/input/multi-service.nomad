# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "multi-service" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "test" {
    network {
      mode = "bridge"
    }

    service {
      name = "echo1"
      port = "9001"

      connect {
        sidecar_service {}
      }
    }

    task "echo1" {
      driver = "docker"

      config {
        image = "hashicorp/http-echo"
        args  = ["-listen=:9001", "-text=echo1"]
      }
    }

    service {
      name = "echo2"
      port = "9002"

      connect {
        sidecar_service {}
      }
    }

    task "echo2" {
      driver = "docker"

      config {
        image = "hashicorp/http-echo"
        args  = ["-listen=:9002", "-text=echo2"]
      }
    }
  }
}
