# node plugins mount the volume on the host and into other tasks.
# when a volume is requested by a task, this plugin gets the volume context
# from nomad that was set by the controller, and uses it to mount NFS to the host,
# then mount that into a task container.
job "node" {
  type = "system"
  group "node" {
    task "node" {
      driver = "docker"
      config {
        image = "democraticcsi/democratic-csi:v1.8.3"
        args = [
          "--csi-version=1.2.0",
          "--csi-name=org.democratic-csi.nfs",
          "--driver-config-file=${NOMAD_TASK_DIR}/driver-config.yaml",
          "--log-level=debug",
          "--csi-mode=node",
          "--server-socket=${CSI_ENDPOINT}",
        ]
        privileged   = true   # node plugins are always privileged to mount disks.
        network_mode = "host" # allows rpc.statd to work for remote NFS locking.
      }
      csi_plugin {
        id   = "org.democratic-csi.nfs" # must match --csi-name
        type = "node"                   # --csi-mode
      }
      template {
        destination = "${NOMAD_TASK_DIR}/driver-config.yaml"
        # minimal required data here, just enough for the plugin to know
        # how to use the Context that the controller adds to the volume.
        data = "driver: nfs-client"
      }
    }
  }
}
