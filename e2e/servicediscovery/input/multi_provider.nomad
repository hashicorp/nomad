# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "service_discovery" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "service_discovery" {

    service {
      name     = "http-api"
      provider = "consul"
      tags     = ["foo", "bar"]
    }

    task "test" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
    }
  }

  group "service_discovery_secondary" {

    service {
      name     = "http-api-nomad"
      provider = "nomad"
      tags     = ["foo", "bar"]
    }

    task "test" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
    }
  }
}
