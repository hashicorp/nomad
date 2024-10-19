# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "juicefs-controller" {
  type        = "system"
  datacenters = ["dc1"]

  group "juicefs-controller" {
    task "juicefs-controller" {
      driver = "docker"

      config {
        image = "juicedata/juicefs-csi-driver:v0.25.0"

        args = [
          "--endpoint=unix://csi/csi.sock",
          "--logtostderr",
          "--nodeid=test",
          "--v=5",
          "--by-process=true"
        ]

        privileged   = true
      }

      csi_plugin {
        id        = "juicefs"
        type      = "controller"
        mount_dir = "/csi"
      }

      resources {
        cpu    = 100
        memory = 512
      }

      env {
        POD_NAME = "csi-controller"
      }
    }
  }
}
