/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default `job "service-discovery-example" {
  // Specifies the datacenter where this job should be run
  // This can be omitted and it will default to ["*"]
  datacenters = ["*"]

  group "client" {
    task "curl" {
      driver = "docker"

      config {
        image   = "curlimages/curl:7.87.0"
        command = "/bin/ash"
        args    = ["local/script.sh"]
      }

      template {
        data        = <<EOF
#!/usr/bin/env ash

while true; do
{{range nomadService "nomad-service-discovery-example-server"}}
  curl -L -v http://{{.Address}}:{{.Port}}/
{{end}}
  sleep 3
done
EOF
        destination = "local/script.sh"
      }

      resources {
        cpu    = 10
        memory = 50
      }
    }
  }

  group "server" {
    network {
      port "www" {
        to = 8001
      }
    }

    task "http" {
      driver = "docker"

      service {
        name     = "nomad-service-discovery-example-server"
        provider = "nomad"
        port     = "www"
        // If you're running Nomad in dev mode, uncomment the following address_mode line to allow this service to be discovered
        // address_mode = "driver"

        check {
          type     = "http"
          path     = "/"
          interval = "5s"
          timeout  = "1s"
        }
      }

      config {
        image   = "busybox:1"
        command = "httpd"
        args    = ["-v", "-f", "-p", "\${NOMAD_PORT_www}", "-h", "/local"]
        ports   = ["www"]
      }

      template {
        data        = <<EOF
hello world
EOF
        destination = "local/index.html"
      }

      resources {
        cpu    = 10
        memory = 50
      }
    }
  }
}`;
