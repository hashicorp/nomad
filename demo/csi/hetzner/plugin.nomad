job "hetzner" {
  datacenters = ["fsn1"]
  type        = "system"

  group "csi" {
    task "plugin" {
      driver = "docker"
      env {
        HCLOUD_TOKEN = "${token}"
        CSI_ENDPOINT = "unix://csi/csi.sock"
      }
      config {
        image = "hetznercloud/hcloud-csi-driver:${hcloud_csi_driver_version}"
        args = [
          "--endpoint=unix://csi/csi.sock",
        ]

        privileged = true
      }

      csi_plugin {
        id        = "hetzner"
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
