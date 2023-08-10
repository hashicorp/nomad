# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "prometheus" {
  datacenters = ["dc1", "dc2"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "monitoring" {
    count = 1

    restart {
      attempts = 2
      interval = "30m"
      delay    = "15s"
      mode     = "fail"
    }

    ephemeral_disk {
      size = 300
    }

    network {
      port "prometheus_ui" {
        to = 9090
      }
    }

    task "prometheus" {
      template {
        change_mode = "noop"
        destination = "local/prometheus.yml"

        data = <<EOH
---
global:
  scrape_interval:     5s
  evaluation_interval: 5s

scrape_configs:

  - job_name: 'nomad_metrics'

    consul_sd_configs:
    - server: '{{ env "NOMAD_IP_prometheus_ui" }}:8500'

    relabel_configs:
    - source_labels: ['__meta_consul_tags']
      regex: '(.*)http(.*)'
      action: keep

    scheme: https
    tls_config:
        ca_file: '/etc/nomad.d/tls/ca.crt'
        cert_file: '/etc/nomad.d/tls/agent.crt'
        key_file: '/etc/nomad.d/tls/agent.key'

    scrape_interval: 5s
    metrics_path: /v1/metrics
    params:
      format: ['prometheus']
EOH

      }

      driver = "docker"

      config {
        image = "prom/prometheus:latest"

        volumes = [
          "local/prometheus.yml:/etc/prometheus/prometheus.yml",
        ]

        # TODO: https://github.com/hashicorp/nomad/issues/11484
        # This is very much not how we should do this, because it
        # exposes the client's mTLS cert to the task and lets the
        # prometheus masquerade as the client.
        mount {
          type     = "bind"
          target   = "/etc/nomad.d/tls"
          source   = "/etc/nomad.d/tls"
          readonly = true
        }

        ports = ["prometheus_ui"]
      }

      service {
        name = "prometheus"
        tags = ["urlprefix-/"]
        port = "prometheus_ui"

        check {
          name     = "prometheus_ui port alive"
          type     = "http"
          path     = "/-/healthy"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
