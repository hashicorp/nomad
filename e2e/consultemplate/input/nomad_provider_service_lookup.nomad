# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "nomad_provider_service_lookup" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "nomad_provider_service_lookup" {

    task "test" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }

      template {
        data = <<EOH
{{ range nomadServices }}
service {{ .Name }} {{ .Tags }}{{ end }}
EOH

        destination = "${NOMAD_TASK_DIR}/services.conf"
        change_mode = "restart"
      }

      template {
        data = <<EOH
{{ range nomadService "default-nomad-provider-service-primary" }}
service {{ .Name }} {{ .Tags }} {{ .Datacenter }} {{ .AllocID }}{{ end }}
EOH

        destination = "${NOMAD_TASK_DIR}/service.conf"
        change_mode = "noop"
      }
    }
  }
}
