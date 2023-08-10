# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# A test NFS server that serves a host volume for persistent state.
job "nfs" {
  group "nfs" {
    service {
      name     = "nfs"
      port     = "nfs"
      provider = "nomad"
    }
    network {
      port "nfs" {
        to     = 2049
        static = 2049
      }
    }
    volume "host-nfs" {
      type   = "host"
      source = "host-nfs"
    }
    task "nfs" {
      driver = "docker"
      config {
        image      = "atlassian/nfs-server-test:2.1"
        ports      = ["nfs"]
        privileged = true
      }
      env {
        # this is the container's default, but being explicit is nice.
        EXPORT_PATH = "/srv/nfs"
      }
      volume_mount {
        volume      = "host-nfs"
        destination = "/srv/nfs"
      }
    }
  }
}
