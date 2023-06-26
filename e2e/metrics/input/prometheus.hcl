# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "prometheus" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "monitoring" {
    update {
      min_healthy_time = "2s"
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    network {
      mode = "host"
      port "ui" {
        static = 9090
      }
    }

    service {
      provider = "nomad"
      name     = "prometheus"
      port     = "ui"
      check {
        type     = "http"
        path     = "/-/healthy"
        interval = "5s"
        timeout  = "2s"
      }
    }

    task "prometheus" {
      driver = "podman"
      user   = "nobody"

      config {
        image        = "docker.io/prom/prometheus:v2.45.0"
        args         = ["--config.file=${NOMAD_TASK_DIR}/config.yaml"]
        network_mode = "host"
      }

      template {
        change_mode = "noop"
        destination = "local/config.yaml"
        data        = <<EOH
global:
  scrape_interval: 2s
  evaluation_interval: 2s

scrape_configs:
  - job_name: 'nomad_metrics'
    nomad_sd_configs:
      - server: 'http://192.168.88.189:4646'
    relabel_configs:
      - source_labels: ['__meta_nomad_tags']
        regex: '(.*)monitor(.*)'
        action: keep
    scheme: http
    metrics_path: '/v1/metrics'
    params:
      format: ['prometheus']
EOH 
      }

      resources {
        cores  = 1
        memory = 512
      }
    }
  }
}
