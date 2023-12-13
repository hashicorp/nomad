# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "helloworld" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "hello" {
    count = 3

    network {
      port "web" {}
    }

    task "hello" {
      driver = "raw_exec"

      config {
        command = "local/hello"
      }

      artifact {
        source      = "https://nomad-community-demo.s3.amazonaws.com/hellov1"
        destination = "local/hello"
        mode        = "file"
      }

      resources {
        cpu    = 500
        memory = 256
      }

      service {
        name = "hello"
        tags = ["urlprefix-hello/"]
        port = "web"

        check {
          name     = "alive"
          type     = "http"
          path     = "/"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
