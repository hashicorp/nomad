# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "caddy" {
  type = "service"

  group "linux" {
    constraint {
      attribute = "${attr.kernel.name}"
      value     = "linux"
    }

    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    update {
      min_healthy_time = "5s"
    }

    network {
      mode = "bridge"
      port "http" {
        static = 9999 # open
        to     = 3000
      }
    }

    service {
      provider = "nomad"
      name     = "caddy"
      port     = "http"
      tags     = ["${attr.unique.platform.aws.public-ipv4}", "expose"]
      check {
        type     = "http"
        path     = "/"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "caddy" {
      driver = "podman"
      user   = "nobody"

      config {
        image = "docker.io/library/caddy:2"
        args  = ["caddy", "run", "--config", "${NOMAD_TASK_DIR}/Caddyfile"]
      }

      template {
        destination = "local/Caddyfile"
        data        = <<EOH
{
  auto_https off
  http_port 3000
}
http:// {
{{ $allocID := env "NOMAD_ALLOC_ID" -}}
{{ range nomadService 1 $allocID "prometheus" }}
  reverse_proxy {{ .Address }}:{{ .Port }}
{{ end }}
}
EOH
      }

      resources {
        cpu    = 200
        memory = 200
      }
    }
  }
}
