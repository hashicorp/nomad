# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "nomad_provider_service" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "nomad_provider_service" {

    service {
      name     = "${NOMAD_NAMESPACE}-nomad-provider-service-primary"
      provider = "nomad"
      tags     = ["foo", "bar"]
    }

    service {
      name     = "${NOMAD_NAMESPACE}-nomad-provider-service-secondary"
      provider = "nomad"
      tags     = ["baz", "buz"]
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
