# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "digitalocean" {

  datacenters = ["dc1"]
  type        = "system"

  group "csi" {
    task "plugin" {
      driver = "docker"


      config {
        image = "digitalocean/do-csi-plugin:v4.12.0"
        entrypoint = [ "ash", "/local/run.sh" ]
        privileged = true
      }
      
      template {
        data        = <<EOH
DO_TOKEN="{{ with nomadVar "secrets/digitalocean_csi_driver" }}{{ .token }}{{ end }}"
EOH
        destination = "secrets/do-token.env"
        env         = true
      }

      template {
        data = <<EOF
#!/usr/bin/env ash
/bin/do-csi-plugin --endpoint="unix:///csi/csi.sock" --token="$DO_TOKEN" --url="https://api.digitalocean.com/" # optionally you can add parameters such as --volume-limit=20
EOF
        destination = "local/run.sh"
        env         = false
      }

      csi_plugin {
        id        = "digitalocean"
        type      = "monolith"
        mount_dir = "/csi"
      }

      resources {
        cpu    = 100
        memory = 64
        memory_max = 128                                                                                       # be aware of using Nomad Scheduler Memory Oversubscription
      }
    }
  }
}
