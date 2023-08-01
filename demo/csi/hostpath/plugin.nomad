job "csi-plugin" {
  type        = "system"
  datacenters = ["dc1"]

  group "csi" {

    task "plugin" {
      driver = "docker"

      config {
        image = "quay.io/k8scsi/hostpathplugin:v1.2.0"

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
