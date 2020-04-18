# jobspec for running CSI plugin for AWS EBS, derived from
# the kubernetes manifests found at
# https://github.com/kubernetes-sigs/aws-ebs-csi-driver/tree/master/deploy/kubernetes

job "plugin-aws-ebs-controller" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "controller" {
    task "plugin" {
      driver = "docker"

      config {
        image = "amazon/aws-ebs-csi-driver:v0.5.0"

        args = [
          "controller",
          "--endpoint=unix://csi/csi.sock",
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
