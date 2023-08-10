# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "simple_lb_clients" {
  datacenters = ["dc1"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "client_1" {
    task "cat" {
      driver = "raw_exec"
      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
      resources {
        cpu    = 10
        memory = 16
      }
      template {
        destination = "output.txt"
        data        = <<EOH
{{$allocID := env "NOMAD_ALLOC_ID" -}}
{{range nomadService 1 $allocID "db"}}
  server {{ .Address }}:{{ .Port }}
{{- end}}
EOH
      }
    }
  }

  group "client_2" {
    task "cat" {
      driver = "raw_exec"
      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
      resources {
        cpu    = 10
        memory = 16
      }
      template {
        destination = "output.txt"
        data        = <<EOH
{{$allocID := env "NOMAD_ALLOC_ID" -}}
{{range nomadService 2 $allocID "db"}}
  server {{ .Address }}:{{ .Port }}
{{- end}}
EOH
      }
    }
  }
}
