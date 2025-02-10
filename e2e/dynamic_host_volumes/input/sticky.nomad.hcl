# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "example" {

  # this job will get deployed and recheduled a lot in this test, so make sure
  # it happens as quickly as possible

  update {
    min_healthy_time = "1s"
  }

  reschedule {
    delay          = "5s"
    delay_function = "constant"
    unlimited      = true
  }

  group "web" {

    network {
      mode = "bridge"
      port "www" {
        to = 8001
      }
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    volume "data" {
      type   = "host"
      source = "sticky-volume"
      sticky = true
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
