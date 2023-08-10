# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "connect_gateway_terminating" {
  group "group" {
    service {
      name = "terminating-gateway-service"

      connect {
        gateway {
          proxy {
            connect_timeout                     = "3s"
            envoy_gateway_bind_tagged_addresses = true

            envoy_gateway_bind_addresses "listener1" {
              address = "10.0.0.1"
              port    = 8888
            }

            envoy_gateway_bind_addresses "listener2" {
              address = "10.0.0.2"
              port    = 8889
            }

            envoy_gateway_no_default_bind = true
            envoy_dns_discovery_type      = "LOGICAL_DNS"

            config {
              foo = "bar"
            }
          }

          terminating {
            service {
              name      = "service1"
              ca_file   = "ca.pem"
              cert_file = "cert.pem"
              key_file  = "key.pem"
            }

            service {
              name = "service2"
              sni  = "myhost"
            }
          }
        }
      }
    }
  }
}
