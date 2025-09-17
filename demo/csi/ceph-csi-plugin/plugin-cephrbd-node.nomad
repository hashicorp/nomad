# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "plugin-cephrbd-node" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  type = "system"

  group "cephrbd" {

    network {
      port "prometheus" {}
    }

    service {
      name     = "prometheus"
      port     = "prometheus"
      provider = "nomad"
      tags     = ["ceph-csi"]
    }

    task "plugin" {
      driver = "docker"

      config {
        image = "quay.io/cephcsi/cephcsi:canary"

        args = [
          "--drivername=rbd.csi.ceph.com",
          "--v=5",
          "--type=cephfs",
          "--nodeserver=true",
          "--nodeid=${NODE_ID}",
          "--instanceid=${POD_ID}",
          "--endpoint=${CSI_ENDPOINT}",
          "--metricsport=${NOMAD_PORT_prometheus}",
        ]

        privileged = true
        ports      = ["prometheus"]


        # TODO: this is... interesting?
        security_opt = ["label=disable"]
      }

      template {
        data = <<-EOT
POD_ID=${NOMAD_ALLOC_ID}
NODE_ID=${node.unique.id}
EOT

        destination = "${NOMAD_TASK_DIR}/env"
        env         = true
      }

      csi_plugin {
        id        = "cephrbd"
        type      = "node"
        mount_dir = "/csi"
      }

      # note: there's no upstream guidance on resource usage so
      # this is a best guess until we profile it in heavy use
      resources {
        cpu    = 256
        memory = 256
      }
    }
  }
}
