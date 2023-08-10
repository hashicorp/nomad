# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "digitalocean" {

  datacenters = ["dc1"]
  type        = "system"

  group "csi" {
    task "plugin" {
      driver = "docker"

      config {
        image = "digitalocean/do-csi-plugin:v2.1.1"
        args = [
          "--endpoint=${CSI_ENDPOINT}",
          "--token=${token}",
          "--url=https://api.digitalocean.com/",
        ]

        privileged = true
      }

      csi_plugin {
        id        = "digitalocean"
        type      = "monolith"
        mount_dir = "/csi"
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
