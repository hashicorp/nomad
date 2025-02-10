# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "example" {

  type = "batch"

  group "web" {

    constraint {
      attribute = "${attr.kernel.name}"
      value     = "linux"
    }

    network {
      mode = "bridge"
      port "www" {
        to = 8001
      }
    }

    volume "data" {
      type   = "host"
      source = "registered-volume"
    }

    task "http" {

      driver = "docker"
      config {
        image   = "busybox:1"
        command = "httpd"
        args    = ["-v", "-f", "-p", "8001", "-h", "/var/www"]
        ports   = ["www"]
      }

      volume_mount {
        volume      = "data"
        destination = "/var/www"
      }

      resources {
        cpu    = 128
        memory = 128
      }

    }
  }
}
