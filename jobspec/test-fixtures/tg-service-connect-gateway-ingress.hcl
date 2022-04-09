job "connect_gateway_ingress" {
  group "group" {
    service {
      name = "ingress-gateway-service"

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
            config {
              foo = "bar"
            }
          }
          ingress {
            tls {
              enabled = true
            }

            listener {
              port     = 8001
              protocol = "tcp"
              service {
                name  = "service1"
                hosts = ["127.0.0.1:8001", "[::1]:8001"]
              }
              service {
                name  = "service2"
                hosts = ["10.0.0.1:8001"]
              }
            }

            listener {
              port     = 8080
              protocol = "http"
              service {
                name  = "nginx"
                hosts = ["2.2.2.2:8080"]
              }
            }
          }
        }
      }
    }
  }
}
