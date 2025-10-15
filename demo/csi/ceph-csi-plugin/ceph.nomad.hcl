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
  description = "hostname of the Ceph container"
}

job "ceph" {

  group "ceph" {

    network {
      mode     = "bridge"
      hostname = var.hostname

      port "ceph_mon" {
        to = 3300
      }
      port "ceph_dashboard" {
        to = 5000
      }
    }

    service {
      name     = "ceph-mon"
      port     = "ceph_mon"
      provider = "nomad"
    }

    service {
      name     = "ceph-dashboard"
      port     = "ceph_dashboard"
      provider = "nomad"

      # TODO: dashboard is never coming up!
      #
      # check {
      #   type           = "http"
      #   interval       = "5s"
      #   timeout        = "1s"
      #   path           = "/"
      #   initial_status = "warning"
      # }
    }

    task "ceph" {
      driver = "docker"

      config {
        # TODO: this should be moved to "quay.io/ceph/ceph:latest" but that
        # image doesn't have the "demo" command
        #image        = "ceph/daemon:latest-octopus"
        image = "quay.io/benjamin_holmes/ceph-aio:v19"
        #args       = ["demo"]
        privileged = true
        ports      = ["ceph_mon", "ceph_dashboard"]

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

      env {
        CEPH_PUBLIC_NETWORK = "0.0.0.0/0"
        CEPH_DEMO_UID       = "demo"
        CEPH_DEMO_BUCKET    = "foobar"
      }

      template {
        data = <<EOT
MON_IP={{ env "NOMAD_ALLOC_IP_ceph_mon" }}
EOT

        destination = "${NOMAD_TASK_DIR}/env"
        env         = true
        once        = true
      }

      template {
        data        = <<EOT
[global]
fsid = ${var.cluster_id}
mon initial members = ${var.hostname}
mon host = v2:{{ env "NOMAD_ALLOC_IP_ceph_mon" }}:3300/0

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
        once        = true
      }
    }
  }
}
