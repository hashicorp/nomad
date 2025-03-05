# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "alloc_count" {
  type    = number
  default = 1
}

job "nfs-controller" {
  group "controller" {
    count = var.alloc_count

    task "controller" {
      driver = "docker"

      config {
        image = "registry.gitlab.com/rocketduck/csi-plugin-nfs:1.1.0"
        args = [
          "--type=controller",
          "--endpoint=${CSI_ENDPOINT}",
          "--node-id=${attr.unique.hostname}",
          "--nfs-server=${NFS_ADDRESS}:/srv/nfs",
          "--log-level=DEBUG",
          "--mount-options=nolock,defaults"
        ]
        privileged = true
      }

      csi_plugin {
        id   = "rocketduck-nfs"
        type = "controller"

        # the NFS workload is launched in parallel and can take a long time to
        # start up
        health_timeout = "2m"
      }

      template {
        data        = "NFS_ADDRESS={{- range nomadService `nfs` }}{{ .Address }}{{ end -}}"
        destination = "local/nfs.addy"
        env         = true
      }
    }
  }
}
