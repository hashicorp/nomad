job "binstore-storagelocker" {
  group "binsl" {
    task "binstore" {
      driver = "docker"

      csi_plugin {
        id        = "org.hashicorp.csi"
        type      = "monolith"
        mount_dir = "/csi/test"
      }
    }
  }
}
