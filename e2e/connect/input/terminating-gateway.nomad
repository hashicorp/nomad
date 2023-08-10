# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "countdash-terminating" {

  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "api" {
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
    }
  }

  group "gateway" {
    network {
      mode = "bridge"
    }

    service {
      name = "api-gateway"

      connect {
        gateway {
          proxy {
            # The following options are automatically set by Nomad if not explicitly
            # configured with using bridge networking.
            #
            # envoy_gateway_no_default_bind = true
            # envoy_gateway_bind_addresses "default" {
            #   address = "0.0.0.0"
            #   port    = <generated listener port>
            # }
            # Additional options are documented at
            # https://www.nomadproject.io/docs/job-specification/gateway#proxy-parameters
          }

          terminating {
            # Nomad will automatically manage the Configuration Entry in Consul
            # given the parameters in the terminating block.
            #
            # Additional options are documented at
            # https://www.nomadproject.io/docs/job-specification/gateway#terminating-parameters
            service {
              name = "count-api"
            }
          }
        }
      }
    }
  }

  group "dashboard" {
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
}
