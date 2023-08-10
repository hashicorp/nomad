# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "multi-ingress" {

  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "gateways" {
    network {
      mode = "bridge"
      port "inbound1" {
        static = 8081
        to     = 8081
      }
      port "inbound2" {
        static = 8082
        to     = 8082
      }
      port "inbound3" {
        static = 8083
        to     = 8083
      }
    }

    service {
      name = "ig1"
      port = "8081"
      connect {
        gateway {
          ingress {
            listener {
              port     = 8081
              protocol = "tcp"
              service {
                name = "api1"
              }
            }
          }
        }
      }
    }

    service {
      name = "ig2"
      port = "8082"
      connect {
        gateway {
          ingress {
            listener {
              port     = 8082
              protocol = "tcp"
              service {
                name = "api2"
              }
            }
          }
        }
      }
    }

    service {
      name = "ig3"
      port = "8083"
      connect {
        gateway {
          ingress {
            listener {
              port     = 8083
              protocol = "tcp"
              service {
                name = "api3"
              }
            }
          }
        }
      }
    }
  }

  group "api1" {
    network {
      mode = "host"
      port "api" {}
    }

    service {
      name = "api1"
      port = "api"

      connect {
        native = true
      }
    }

    task "api1" {
      driver = "docker"

      config {
        image        = "hashicorpdev/uuid-api:v5"
        network_mode = "host"
      }

      env {
        BIND = "0.0.0.0"
        PORT = "${NOMAD_PORT_api}"
      }
    }
  }

  group "api2" {
    network {
      mode = "host"
      port "api" {}
    }

    service {
      name = "api2"
      port = "api"

      connect {
        native = true
      }
    }

    task "api2" {
      driver = "docker"

      config {
        image        = "hashicorpdev/uuid-api:v5"
        network_mode = "host"
      }

      env {
        BIND = "0.0.0.0"
        PORT = "${NOMAD_PORT_api}"
      }
    }
  }

  group "api3" {
    network {
      mode = "host"
      port "api" {}
    }

    service {
      name = "api3"
      port = "api"

      connect {
        native = true
      }
    }

    task "api3" {
      driver = "docker"

      config {
        image        = "hashicorpdev/uuid-api:v5"
        network_mode = "host"
      }

      env {
        BIND = "0.0.0.0"
        PORT = "${NOMAD_PORT_api}"
      }
    }
  }
}
