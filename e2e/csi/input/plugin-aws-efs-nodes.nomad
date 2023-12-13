# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# jobspec for running CSI plugin for AWS EFS, derived from
# the kubernetes manifests found at
# https://github.com/kubernetes-sigs/aws-efs-csi-driver/tree/master/deploy/kubernetes

job "plugin-aws-efs-nodes" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  # you can run node plugins as service jobs as well, but this ensures
  # that all nodes in the DC have a copy.
  type = "system"

  group "nodes" {
    task "plugin" {
      driver = "docker"

      config {
        image = "amazon/aws-efs-csi-driver:v1.3.6"
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

      # note: there's no upstream guidance on resource usage so
      # this is a best guess until we profile it in heavy use
      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
