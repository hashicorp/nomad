# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "tensorrt" {
  datacenters = ["dc1"]

  group "back" {
    task "rtserver" {
      driver = "docker"
      config {
        image   = "nvcr.io/nvidia/tensorrtserver:19.02-py3"
        command = "trtserver"
        args = [
          "--model-store=${NOMAD_TASK_DIR}/models"
        ]
        shm_size = 1024
        port_map {
          http    = 8000
          grpc    = 8001
          metrics = 8002
        }
        ulimit {
          memlock = "-1"
          stack   = "67108864"
        }
      }

      service {
        port = "http"
        tags = ["http"]
        check {
          type     = "http"
          port     = "http"
          path     = "/api/health/ready"
          interval = "5s"
          timeout  = "1s"
        }
        check_restart {
          grace = "30s"
        }
      }

      # load the example model into ${NOMAD_TASK_DIR}/models
      artifact {
        source      = "http://download.caffe2.ai.s3.amazonaws.com/models/resnet50/predict_net.pb"
        destination = "local/models/resnet50_netdef/1/model.netdef"
        mode        = "file"
      }
      artifact {
        source      = "http://download.caffe2.ai.s3.amazonaws.com/models/resnet50/init_net.pb"
        destination = "local/models/resnet50_netdef/1/init_model.netdef"
        mode        = "file"
      }
      artifact {
        source      = "https://raw.githubusercontent.com/NVIDIA/tensorrt-inference-server/v1.0.0/docs/examples/model_repository/resnet50_netdef/config.pbtxt"
        destination = "local/models/resnet50_netdef/config.pbtxt"
        mode        = "file"
      }
      artifact {
        source      = "https://raw.githubusercontent.com/NVIDIA/tensorrt-inference-server/v1.0.0/docs/examples/model_repository/resnet50_netdef/resnet50_labels.txt"
        destination = "local/models/resnet50_netdef/resnet50_labels.txt"
        mode        = "file"
      }

      resources {
        cpu    = 8192
        memory = 8192
        network {
          mbits = 10
          port "http" {}
        }

        # an Nvidia GPU with >= 4GiB memory, preferably a Tesla
        device "nvidia/gpu" {
          count = 1
          constraint {
            attribute = "${device.attr.memory}"
            operator  = ">="
            value     = "4 GiB"
          }
          affinity {
            attribute = "${device.model}"
            operator  = "regexp"
            value     = "Tesla"
          }
        }
      }
    }
  }

  group "front" {
    task "web" {

      driver = "docker"

      config {
        image = "nvidia/tensorrt-labs:frontend"
        args = [
          "main.py", "${RTSERVER}"
        ]
        port_map {
          http = 5000
        }
      }

      resources {
        cpu    = 1024
        memory = 1024
        network {
          mbits = 10
          port "http" { static = "8888" }
        }
      }

      template {
        data        = <<EOH
          RTSERVER = {{ with service "tensorrt-back-rtserver" }}{{ with index . 0 }} http://{{.Address }}:{{.Port }} {{ end }}{{ end }}
        EOH
        destination = "local/rtserver.env"
        env         = true
      }

    }
  }

}
