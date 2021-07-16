job "plugin-aws-efs-nodes" { 
  datacenters = ["dc1"]

  # Curently amd64 only, arm64 hopefully coming soon.
  # Follow https://github.com/kubernetes-sigs/aws-efs-csi-driver/issues/111
  constraint {
     attribute = "${attr.cpu.arch}"
     value     = "amd64"
  }

  type = "system"

  group "nodes" {
    ephemeral_disk {
      size = "128"
    }

    task "plugin" {
      driver = "docker"

      config {
        image = "amazon/aws-efs-csi-driver:v1.0.0"

	args = [ 
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
        id        = "aws-efs"
        type      = "node"
        mount_dir = "/csi"
      }

      resources {
        cpu    = 100
        memory = 32
      }
    }
  }
}
