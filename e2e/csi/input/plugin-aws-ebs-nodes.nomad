# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# jobspec for running CSI plugin for AWS EBS, derived from
# the kubernetes manifests found at
# https://github.com/kubernetes-sigs/aws-ebs-csi-driver/tree/master/deploy/kubernetes

job "plugin-aws-ebs-nodes" {
  datacenters = ["dc1", "dc2"]

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
        image = "public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.5.1"

        args = [
          "node",
          "--endpoint=${CSI_ENDPOINT}",
          "--logtostderr",
          "--v=5",
        ]

        privileged = true
      }

      csi_plugin {
        id        = "aws-ebs0"
        type      = "node"
        mount_dir = "/csi"
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
