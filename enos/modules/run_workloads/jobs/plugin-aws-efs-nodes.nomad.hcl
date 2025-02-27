# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# this variable is not used but required by runner
variable "alloc_count" {
  type    = number
  default = 1
}

job "plugin-aws-efs-nodes" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  type = "system"

  group "nodes" {
    task "plugin" {
      driver = "docker"

      config {
        image = "public.ecr.aws/efs-csi-driver/amazon/aws-efs-csi-driver:v2.1.6"

        args = [
          "node",
          "--endpoint=${CSI_ENDPOINT}",
          "--logtostderr",
          "--v=5",
        ]

        privileged = true
      }

      # note: the EFS driver doesn't seem to respect the --endpoint
      # flag or CSI_ENDPOINT env var and always sets up the listener
      # at '/tmp/csi.sock'
      csi_plugin {
        id        = "aws-efs0"
        type      = "node"
        mount_dir = "/tmp"
      }

      resources {
        cpu    = 100
        memory = 256
      }
    }
  }
}
