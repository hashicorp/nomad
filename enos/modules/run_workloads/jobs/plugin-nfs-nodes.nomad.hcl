# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# this variable is not used but required by runner
variable "alloc_count" {
  type    = number
  default = 1
}

job "nfs-node" {
  type = "system"

  group "node" {
    task "node" {
      driver = "docker"

      config {
        image = "registry.gitlab.com/rocketduck/csi-plugin-nfs:0.6.1"
        args = [
          "--type=node",
          "--endpoint=${CSI_ENDPOINT}",
          "--node-id=${attr.unique.hostname}",
          "--nfs-server=${NFS_ADDRESS}:/srv/nfs",
          "--log-level=DEBUG",
          "--mount-options=nolock,defaults"
        ]

        privileged   = true
        network_mode = "host"
      }

      csi_plugin {
        id   = "rocketduck-nfs"
        type = "node"
      }

      template {
        data        = "NFS_ADDRESS={{- range nomadService `nfs` }}{{ .Address }}{{ end -}}"
        destination = "local/nfs.addy"
        env         = true
      }
    }
  }
}
