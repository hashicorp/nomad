# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# Controller plugins create and manage CSI volumes.
# This one just creates folders within the NFS mount.
job "controller" {
  group "controller" {
    # count = 2 # usually you want a couple controllers for redundancy
    task "controller" {
      driver = "docker"
      csi_plugin {
        id   = "rocketduck-nfs"
        type = "controller"
      }
      config {
        # thanks rocketDuck for aiming directly at Nomad :)
        # https://gitlab.com/rocketduck/csi-plugin-nfs
        image = "registry.gitlab.com/rocketduck/csi-plugin-nfs:0.6.1"
        args = [
          "--type=controller",
          "--endpoint=${CSI_ENDPOINT}", # provided by csi_plugin{}
          "--node-id=${attr.unique.hostname}",
          "--nfs-server=${NFS_ADDRESS}:/srv/nfs",
          "--log-level=DEBUG",
        ]
        privileged = true # this particular controller mounts NFS in itself
      }
      template {
        data        = "NFS_ADDRESS={{- range nomadService `nfs` }}{{ .Address }}{{ end -}}"
        destination = "local/nfs.addy"
        env         = true
      }
    }
  }
}
