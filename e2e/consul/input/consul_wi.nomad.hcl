# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "example" {

  group "example" {
    network {
      port "db" {
        to = 5678
      }
    }

    task "example" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "nc"
        args    = ["-ll", "-p", "1234", "-e", "/bin/cat"]

        ports = ["db"]
      }

      identity {
        name = "consul_default"
        aud  = ["consul.io"]
      }

      consul {}

      template {
        data        = <<-EOT
          CONSUL_TOKEN={{ env "CONSUL_TOKEN" }}
        EOT
        destination = "local/config.txt"
      }

      resources {
        cpu    = 100
        memory = 100
      }

      service {
        name = "consul-example"
        tags = ["global", "cache"]
        port = "db"

        check {
          name     = "alive"
          type     = "tcp"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
