job "plugin-aws-ebs-nodes" { 
  datacenters = ["dc1"]

  type = "system"

  group "nodes" {
    ephemeral_disk {
      size = "128"
    }

    task "plugin" {
      driver = "docker"

      config {
        image = "amazon/aws-ebs-csi-driver:v0.7.0"

	args = [ 
          "node",
          "--endpoint=unix://csi/csi.sock",
          "--logtostderr",
          "--v=5"
        ]

        # If you have blocked access to the EC2 Metadata Service for
        # tasks running in the default "bridge" mode (advisable),
        # allow access by running in host mode.

        network_mode = "host"

        # node plugins must run as privileged jobs because they
        # mount disks to the host

        privileged = true
      }

      csi_plugin {
        id        = "aws-ebs"
        type      = "node"
        mount_dir = "/csi"
      }

      resources {
        cpu    = 100
        memory = 32
      }

      vault {
        policies = ["ebs-csi"]

        change_mode = "restart"
      }

      template {
        data = <<__EOF__
{{ with secret "aws/creds/ebs-csi" -}}
AWS_ACCESS_KEY_ID="{{.Data.access_key}}"
AWS_SECRET_ACCESS_KEY="{{.Data.secret_key}}"
{{ end }}
__EOF__

        destination = "secrets/file.env"
        env         = true
      }
    }
  }
}
