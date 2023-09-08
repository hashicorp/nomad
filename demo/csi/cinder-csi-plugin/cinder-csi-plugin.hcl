# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "cinder-csi-plugin" {
  datacenters = ["dc1"]
  type        = "system"
  group "nodes" {
    vault {
      policies    = ["openstack-secrets-read"]
      change_mode = "restart"
    }
    task "cinder-node" {
      driver = "docker"
      template {
        data        = <<EOF
[Global]
username = {{ with secret "kv/data/openstack/credentials"}}{{ .Data.data.username }}{{ end }}
password =  {{ with secret "kv/data/openstack/credentials"}}{{ .Data.data.password }}{{ end }}
domain-name = default
auth-url = https://service01a-c2.example.com:5001/
tenant-id = 5sd6f4s5df6sd6fs5ds65fd4f65s
region = RegionOne
EOF
        destination = "local/cloud.conf"
        change_mode = "restart"
      }
      config {
        image = "docker.io/k8scloudprovider/cinder-csi-plugin:latest"
        devices = [{
          host_path      = "/dev"
          container_path = "/dev"
        }]
        volumes = [
          "./local/cloud.conf:/etc/config/cloud.conf"
        ]

        args = [
          "/bin/cinder-csi-plugin",
          "-v=4",
          "--endpoint=${CSI_ENDPOINT}",
          "--cloud-config=/etc/config/cloud.conf",
          "--nodeid=${node.unique.name}",
        ]
        privileged = true
      }

      csi_plugin {
        id        = "cinder-csi"
        type      = "node"
        mount_dir = "/csi"
      }
    }
    task "cinder-controller" {

      template {
        data        = <<EOF
[Global]
username = {{ with secret "kv/data/openstack/credentials"}}{{ .Data.data.username }}{{ end }}
password =  {{ with secret "kv/data/openstack/credentials"}}{{ .Data.data.password }}{{ end }}
domain-name = default
auth-url = https://service01a-c2.example.com:5001/
tenant-id = asdfasdfasdfa09asd8fa09sdf8009as8df0sa98
region = RegionOne
EOF
        destination = "local/cloud.conf"
        change_mode = "restart"
      }
      driver = "docker"

      config {
        image = "docker.io/k8scloudprovider/cinder-csi-plugin:latest"
        volumes = [
          "./local/cloud.conf:/etc/config/cloud.conf"
        ]

        args = [
          "/bin/cinder-csi-plugin",
          "-v=4",
          "--endpoint=${CSI_ENDPOINT}",
          "--cloud-config=/etc/config/cloud.conf",
          "--nodeid=${node.unique.name}",
          "--cluster=${NOMAD_DC}"
        ]
      }

      csi_plugin {
        id        = "cinder-csi"
        type      = "controller"
        mount_dir = "/csi"
      }
    }
  }
}
