// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
        to = 9002
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
        image          = "hashicorpdev/counter-dashboard:v3"
        auth_soft_fail = true
      }


      # this template can't be used for the COUNTING_SERVICE_URL because it
      # needs the Nomad-assigned upstream address here and not the Consul
      # service address, but this is handy for testing.
      template {
        data = <<EOT
{{- range service "count-api" }}
API_ADDR=http://{{ .Address }}:{{ .Port }}{{- end }}
EOT

        destination = "local/count-api.txt"
      }

    }
  }
}
