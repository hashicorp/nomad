# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "juicefs-node" {
  type        = "system"
  datacenters = ["dc1"]

  group "juicefs-node" {
    network {
      mode = "host"
    }

    task "juicefs-node" {
      driver       = "docker"

      config {
        network_mode = "host"
        privileged = true

        image = "juicedata/juicefs-csi-driver:v0.25.0"

        args = [
          "--endpoint=unix://csi/csi.sock",
          "--logtostderr",
          "--v=5",
          "--nodeid=test",
          "--by-process=true",
        ]

        volumes = [
          # If you have a dedicated directory for JuiceFS cache, you can mount it here into the container.
          "/var/jfsCache:/var/jfsCache",
        ]
      }

      csi_plugin {
        id        = "juicefs"
        type      = "node"
        mount_dir = "/csi"
      }

      resources {
        cpu    = 100
        memory = 1024
      }

      env {
        POD_NAME = "csi-node"

        # Set MINIO_REGION if you are using MinIO as the JuiceFS backend.
        #MINIO_REGION = "us-west"
      }
    }
  }
}
