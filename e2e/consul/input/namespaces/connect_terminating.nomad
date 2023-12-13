# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "connect_terminating" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "api" {

    consul {
      namespace = "apple"
    }

    network {
      mode = "host"
      port "port" {
        static = "9001"
      }
    }

    service {
      name = "count-api"
      port = "port"
    }

    task "api" {
      driver = "docker"

      config {
        image        = "hashicorpdev/counter-api:v3"
        network_mode = "host"
      }

      env {
        PORT = "${NOMAD_PORT_port}"
      }
    }
  }

  group "api-z" {

    # consul namespace not set

    network {
      mode = "host"
      port "port" {
        static = "9011"
      }
    }

    service {
      name = "count-api-z"
      port = "port"
    }

    task "api" {
      driver = "docker"

      config {
        image        = "hashicorpdev/counter-api:v3"
        network_mode = "host"
      }

      env {
        PORT = "${NOMAD_PORT_port}"
      }
    }
  }

  group "gateway" {

    consul {
      namespace = "apple"
    }

    network {
      mode = "bridge"
    }

    service {
      name = "api-gateway"

      connect {
        gateway {
          terminating {
            service {
              name = "count-api"
            }
          }
        }
      }
    }
  }

  group "gateway-z" {

    # consul namespace not set

    network {
      mode = "bridge"
    }

    service {
      name = "api-gateway-z"

      connect {
        gateway {
          terminating {
            service {
              name = "count-api-z"
            }
          }
        }
      }
    }
  }

  group "dashboard" {

    consul {
      namespace = "apple"
    }

    network {
      mode = "bridge"

      port "http" {
        static = 9002
        to     = 9002
      }
    }

    service {
      name = "count-dashboard"
      port = "9002"

      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "count-api"
              local_bind_port  = 8080
            }
          }
        }
      }
    }

    task "dashboard" {
      driver = "docker"

      env {
        COUNTING_SERVICE_URL = "http://${NOMAD_UPSTREAM_ADDR_count_api}"
      }

      config {
        image = "hashicorpdev/counter-dashboard:v3"
      }
    }
  }

  group "dashboard-z" {

    # consul namespace not set

    network {
      mode = "bridge"

      port "http" {
        static = 9012
        to     = 9002
      }
    }

    service {
      name = "count-dashboard-z"
      port = "9012"

      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "count-api-z"
              local_bind_port  = 8080
            }
          }
        }
      }
    }

    task "dashboard" {
      driver = "docker"

      env {
        COUNTING_SERVICE_URL = "http://${NOMAD_UPSTREAM_ADDR_count_api-z}"
      }

      config {
        image = "hashicorpdev/counter-dashboard:v3"
      }
    }
  }
}
