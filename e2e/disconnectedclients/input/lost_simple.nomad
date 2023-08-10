# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "lost_simple" {

  datacenters = ["dc1", "dc2"]

  group "group" {

    count = 2

    constraint {
      attribute = "${attr.kernel.name}"
      value     = "linux"
    }

    constraint {
      operator = "distinct_hosts"
      value    = "true"
    }

    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "httpd"
        args    = ["-v", "-f", "-p", "8001", "-h", "/var/www"]
      }

      resources {
        cpu    = 128
        memory = 128
      }
    }
  }
}
