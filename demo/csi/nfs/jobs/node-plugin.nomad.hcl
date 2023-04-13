# Node plugins mount the volume on the host to present to other tasks.
job "node" {
  # node plugins should run anywhere your task might be placed, i.e. ~everywhere
  type = "system"

  group "node" {
    task "node" {
      driver = "docker"
      csi_plugin {
        id   = "rocketduck-nfs"
        type = "node"
      }
      config {
        # thanks rocketDuck for aiming directly at Nomad :)
        # https://gitlab.com/rocketduck/csi-plugin-nfs
        image = "registry.gitlab.com/rocketduck/csi-plugin-nfs:0.6.1"
        args = [
          "--type=node",
          "--endpoint=${CSI_ENDPOINT}", # provided by csi_plugin{}
          "--node-id=${attr.unique.hostname}",
          "--nfs-server=${NFS_ADDRESS}:/srv/nfs",
          "--log-level=DEBUG",
        ]
        privileged   = true   # node plugins are always privileged to mount disks.
        network_mode = "host" # allows rpc.statd to work for remote NFS locking
      }
      template {
        data        = "NFS_ADDRESS={{- range nomadService `nfs` }}{{ .Address }}{{ end -}}"
        destination = "local/nfs.addy"
        env         = true
      }
    }
  }
}
