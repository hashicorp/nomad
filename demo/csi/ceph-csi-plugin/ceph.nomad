# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# This job deploys Ceph as a Docker container in "demo mode"; it runs all its
# processes in a single task and doesn't will not persist data after a restart

variable "cluster_id" {
  type = string
  # generated from uuid5(dns) with ceph.example.com as the seed
  default     = "e9ba69fa-67ff-5920-b374-84d5801edd19"
  description = "cluster ID for the Ceph monitor"
}

variable "hostname" {
  type        = string
  default     = "linux" # hostname of the Nomad repo's Vagrant box
  description = "hostname of the demo host"
}

job "ceph" {
  datacenters = ["dc1"]

  group "ceph" {

    network {
      # we can't configure networking in a way that will both satisfy the Ceph
      # monitor's requirement to know its own IP address *and* be routable
      # between containers, without either CNI or fixing
      # https://github.com/hashicorp/nomad/issues/9781
      #
      # So for now we'll use host networking to keep this demo understandable.
      # That also means the controller plugin will need to use host addresses.
      mode = "host"
    }

    service {
      name = "ceph-mon"
      port = 3300
    }

    service {
      name = "ceph-dashboard"
      port = 5000

      check {
        type           = "http"
        interval       = "5s"
        timeout        = "1s"
        path           = "/"
        initial_status = "warning"
      }
    }

    task "ceph" {
      driver = "docker"

      config {
        image        = "ceph/daemon:latest-octopus"
        args         = ["demo"]
        network_mode = "host"
        privileged   = true

        mount {
          type   = "bind"
          source = "local/ceph"
          target = "/etc/ceph"
        }
      }

      resources {
        memory = 512
        cpu    = 256
      }

      template {

        data = <<EOT
MON_IP={{ sockaddr "with $ifAddrs := GetDefaultInterfaces | include \"type\" \"IPv4\" | limit 1 -}}{{- range $ifAddrs -}}{{ attr \"address\" . }}{{ end }}{{ end " }}
CEPH_PUBLIC_NETWORK=0.0.0.0/0
CEPH_DEMO_UID=demo
CEPH_DEMO_BUCKET=foobar
EOT


        destination = "${NOMAD_TASK_DIR}/env"
        env         = true
      }

      template {
        data        = <<EOT
[global]
fsid = ${var.cluster_id}
mon initial members = ${var.hostname}
mon host = v2:{{ sockaddr "with $ifAddrs := GetDefaultInterfaces | include \"type\" \"IPv4\" | limit 1 -}}{{- range $ifAddrs -}}{{ attr \"address\" . }}{{ end }}{{ end " }}:3300/0

osd crush chooseleaf type = 0
osd journal size = 100
public network = 0.0.0.0/0
cluster network = 0.0.0.0/0
osd pool default size = 1
mon warn on pool no redundancy = false
osd_memory_target =  939524096
osd_memory_base = 251947008
osd_memory_cache_min = 351706112
osd objectstore = bluestore

[osd.0]
osd data = /var/lib/ceph/osd/ceph-0


[client.rgw.linux]
rgw dns name = ${var.hostname}
rgw enable usage log = true
rgw usage log tick interval = 1
rgw usage log flush threshold = 1
rgw usage max shards = 32
rgw usage max user shards = 1
log file = /var/log/ceph/client.rgw.linux.log
rgw frontends = beast  endpoint=0.0.0.0:8080

EOT
        destination = "${NOMAD_TASK_DIR}/ceph/ceph.conf"
      }
    }
  }
}
