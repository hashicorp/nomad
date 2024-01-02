# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "connect_ingress" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "ingress-group" {

    consul {
      namespace = "apple"
    }

    network {
      mode = "bridge"
      port "inbound" {
        static = 8080
        to     = 8080
      }
    }

    service {
      name = "my-ingress-service"
      port = "8080"

      connect {
        gateway {
          ingress {
            listener {
              port     = 8080
              protocol = "tcp"
              service {
                name = "uuid-api"
              }
            }
          }
        }
      }
    }
  }

  group "ingress-group-z" {

    # consul namespace not set

    network {
      mode = "bridge"
      port "inbound" {
        static = 8081
        to     = 8080
      }
    }

    service {
      name = "my-ingress-service-z"
      port = "8081"

      connect {
        gateway {
          ingress {
            listener {
              port     = 8080
              protocol = "tcp"
              service {
                name = "uuid-api-z"
              }
            }
          }
        }
      }
    }
  }

  group "generator" {

    consul {
      namespace = "apple"
    }

    network {
      mode = "host"
      port "api" {}
    }

    service {
      name = "uuid-api"
      port = "${NOMAD_PORT_api}"

      connect {
        native = true
      }
    }

    task "generate" {
      driver = "docker"

      config {
        image        = "hashicorpdev/uuid-api:v3"
        network_mode = "host"
      }

      env {
        BIND = "0.0.0.0"
        PORT = "${NOMAD_PORT_api}"
      }
    }
  }

  group "generator-z" {

    # consul namespace not set

    network {
      mode = "host"
      port "api" {}
    }

    service {
      name = "uuid-api-z"
      port = "${NOMAD_PORT_api}"

      connect {
        native = true
      }
    }

    task "generate-z" {
      driver = "docker"

      config {
        image        = "hashicorpdev/uuid-api:v3"
        network_mode = "host"
      }

      env {
        BIND = "0.0.0.0"
        PORT = "${NOMAD_PORT_api}"
      }
    }
  }
}
