# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "csi-plugin" {
  type        = "system"
  datacenters = ["dc1"]

  group "csi" {

    task "plugin" {
      driver = "docker"

      config {
        image = "registry.k8s.io/sig-storage/hostpathplugin:v1.9.0"

        args = [
          "--drivername=csi-hostpath",
          "--v=5",
          "--endpoint=${CSI_ENDPOINT}",
          "--nodeid=node-${NOMAD_ALLOC_INDEX}",
        ]

        privileged = true
      }

      csi_plugin {
        id        = "hostpath-plugin0"
        type      = "monolith" #node" # doesn't support Controller RPCs
        mount_dir = "/csi"
      }

      resources {
        cpu    = 256
        memory = 128
      }
    }
  }
}
