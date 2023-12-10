# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# jobspec for running CSI plugin for AWS EBS, derived from
# the kubernetes manifests found at
# https://github.com/kubernetes-sigs/aws-ebs-csi-driver/tree/master/deploy/kubernetes

job "plugin-aws-ebs-controller" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  spread {
    attribute = "${node.unique.id}"
  }

  group "controller" {

    count = 2 // HA for node drain testing

    task "plugin" {
      driver = "docker"

      config {
        image = "public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.5.1"

        args = [
          "controller",
          "--endpoint=${CSI_ENDPOINT}",
          "--logtostderr",
          "--v=5",
        ]

        # note: plugins running as controllers don't
        # need to run as privileged tasks
      }

      csi_plugin {
        id        = "aws-ebs0"
        type      = "controller"
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
