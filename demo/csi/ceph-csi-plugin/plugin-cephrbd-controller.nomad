# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variable "cluster_id" {
  type = string
  # generated from uuid5(dns) with ceph.example.com as the seed
  default     = "e9ba69fa-67ff-5920-b374-84d5801edd19"
  description = "cluster ID for the Ceph monitor"
}

job "plugin-cephrbd-controller" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  type = "service"

  group "cephrbd" {

    network {
      port "prometheus" {}
    }

    service {
      name = "prometheus"
      port = "prometheus"
      tags = ["ceph-csi"]
    }

    task "plugin" {
      driver = "docker"

      config {
        image = "quay.io/cephcsi/cephcsi:canary"

        args = [
          "--drivername=rbd.csi.ceph.com",
          "--v=5",
          "--type=rbd",
          "--controllerserver=true",
          "--nodeid=${NODE_ID}",
          "--instanceid=${POD_ID}",
          "--endpoint=${CSI_ENDPOINT}",
          "--metricsport=${NOMAD_PORT_prometheus}",
        ]

        ports = ["prometheus"]

        # we need to be able to write key material to disk in this location
        mount {
          type     = "bind"
          source   = "secrets"
          target   = "/tmp/csi/keys"
          readonly = false
        }

        mount {
          type     = "bind"
          source   = "ceph-csi-config/config.json"
          target   = "/etc/ceph-csi-config/config.json"
          readonly = false
        }

      }

      template {
        data = <<-EOT
POD_ID=${NOMAD_ALLOC_ID}
NODE_ID=${node.unique.id}
CSI_ENDPOINT=unix://csi/csi.sock
EOT

        destination = "${NOMAD_TASK_DIR}/env"
        env         = true
      }

      # ceph configuration file
      template {

        data = <<EOF
[{
    "clusterID": "${var.cluster_id}",
    "monitors": [
        {{range $index, $service := service "ceph-mon"}}{{if gt $index 0}}, {{end}}"{{.Address}}"{{end}}
    ]
}]
EOF

        destination = "ceph-csi-config/config.json"
      }

      csi_plugin {
        id        = "cephrbd"
        type      = "controller"
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
