job "ceph-csi-plugin" {
  datacenters = ["dc1"]
  type        = "system"
  group "nodes" {
    task "ceph-node" {
      driver = "docker"
      template {
        data        = <<EOF
[{
    "clusterID": "<clusterid>",
    "monitors": [
        {{range $index, $service := service "mon.ceph"}}{{if gt $index 0}}, {{end}}"{{.Address}}"{{end}}
    ]
}]
EOF
        destination = "local/config.json"
        change_mode = "restart"
      }
      config {
        image = "quay.io/cephcsi/cephcsi:v2.1.2-amd64"
        volumes = [
          "./local/config.json:/etc/ceph-csi-config/config.json"
        ]
        mounts = [
          {
            type     = "tmpfs"
            target   = "/tmp/csi/keys"
            readonly = false
            tmpfs_options {
              size = 1000000 # size in bytes
            }
          }
        ]
        args = [
          "--type=rbd",
          # Name of the driver
          "--drivername=rbd.csi.ceph.com",
          "--logtostderr",
          "--nodeserver=true",
          "--endpoint=unix://csi/csi.sock",
          "--instanceid=${attr.unique.platform.aws.instance-id}",
          "--nodeid=${attr.unique.consul.name}",
          # TCP port for liveness metrics requests (/metrics)
          "--metricsport=${NOMAD_PORT_prometheus}",
        ]
        privileged = true
        resources {
          cpu    = 200
          memory = 500
          network {
            mbits = 1
            // prometheus metrics port
            port "prometheus" {}
          }
        }
      }
      service {
        name = "prometheus"
        port = "prometheus"
        tags = ["ceph-csi"]
      }
      csi_plugin {
        id        = "ceph-csi"
        type      = "node"
        mount_dir = "/csi"
      }
    }
    task "ceph-controller" {

      template {
        data        = <<EOF
[{
    "clusterID": "<clusterid>",
    "monitors": [
        {{range $index, $service := service "mon.ceph"}}{{if gt $index 0}}, {{end}}"{{.Address}}"{{end}}
    ]
}]
EOF
        destination = "local/config.json"
        change_mode = "restart"
      }
      driver = "docker"
      config {
        image = "quay.io/cephcsi/cephcsi:v2.1.2-amd64"
        volumes = [
          "./local/config.json:/etc/ceph-csi-config/config.json"
        ]
        resources {
          cpu    = 200
          memory = 500
          network {
            mbits = 1
            // prometheus metrics port
            port "prometheus" {}
          }
        }
        args = [
          "--type=rbd",
          "--controllerserver=true",
          "--drivername=rbd.csi.ceph.com",
          "--logtostderr",
          "--endpoint=unix://csi/csi.sock",
          "--metricsport=$${NOMAD_PORT_prometheus}",
          "--nodeid=$${attr.unique.platform.aws.hostname}"
        ]
      }
      service {
        name = "prometheus"
        port = "prometheus"
        tags = ["ceph-csi"]
      }
      csi_plugin {
        id        = "ceph-csi"
        type      = "controller"
        mount_dir = "/csi"
      }
    }
  }
}