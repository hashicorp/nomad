# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "bridge-ipv6" {
  group "g" {
    network {
      mode = "bridge"
      port "http" {
        to = 8000
        static = 8000
      }

    }
    service {
      name     = "a-web"
      port     = "http"
      provider = "nomad"
      address_mode = "alloc"
    }
    task "t" {
      template {
        data = <<EOH
         #!/usr/bin/env bash

         apt update
         apt install curl -y
         EOH
         destination = "local/bridge_ipv6.sh"
      }
      template {
              data = "hellooo-thereee:P"
              destination = "local/index.html"
            }

      driver = "docker"
      config {
        image   = "python:slim"
        command = "python3"
        args    = ["-m", "http.server", "--directory=local", "--bind=::"]
        ports   = ["http"]
      }
    }
  }
}
