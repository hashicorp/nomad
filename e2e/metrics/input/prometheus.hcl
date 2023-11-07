# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "prometheus" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "monitoring" {
    update {
      min_healthy_time = "5s"
    }

    reschedule {
      attempts = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    network {
      mode = "bridge"
      port "ui" {
        to = 9090
      }
    }

    service {
      provider = "nomad"
      name     = "prometheus"
      port     = "ui"
      tags     = ["e2emetrics"]
      check {
        type     = "http"
        path     = "/-/healthy"
        interval = "5s"
        timeout  = "2s"
      }
    }

    # run a private holepunch instance in this group network
    # so prometheus can access the nomad api for service disco
    task "sidepunch" {
      driver = "podman"
      user   = "nobody"
      config {
        image = "ghcr.io/shoenig/nomad-holepunch:v0.1.4"
      }

      lifecycle {
        hook    = "prestart"
        sidecar = true
      }

      env {
        HOLEPUNCH_BIND      = "127.0.0.1"
        HOLEPUNCH_PORT      = 6666
        HOLEPUNCH_ALLOW_ALL = true # service discovery
      }

      identity {
        env = true
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }

    task "prometheus" {
      driver = "podman"
      user   = "nobody"

      config {
        image = "docker.io/prom/prometheus:v2.45.0"
        args  = ["--config.file=${NOMAD_TASK_DIR}/config.yaml"]
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
      - server: 'http://127.0.0.1:6666'
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

