# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "nomad-proxy" {
  datacenters = ["dc1", "dc2"]
  namespace   = "proxy"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "proxy" {

    network {
      port "www" {
        static = 6464
        to     = 443
      }
    }

    task "nginx" {

      driver = "docker"

      config {
        image = "nginx:latest"
        ports = ["www"]

        mount {
          type   = "bind"
          source = "local/nginx.conf"
          target = "/etc/nginx/nginx.conf"
        }

        mount {
          type   = "bind"
          source = "/etc/nomad.d/tls/tls_proxy.key"
          target = "/etc/ssl/tls_proxy.key"
        }

        mount {
          type   = "bind"
          source = "/etc/nomad.d/tls/tls_proxy.crt"
          target = "/etc/ssl/tls_proxy.crt"
        }

        mount {
          type   = "bind"
          source = "/etc/nomad.d/tls/self_signed.key"
          target = "/etc/ssl/self_signed.key"
        }

        mount {
          type   = "bind"
          source = "/etc/nomad.d/tls/self_signed.crt"
          target = "/etc/ssl/self_signed.crt"
        }
      }

      resources {
        cpu    = 256
        memory = 128
      }

      # this template is mostly lifted from the Learn Guide:
      # https://learn.hashicorp.com/tutorials/nomad/reverse-proxy-ui
      template {
        destination = "local/nginx.conf"
        data        = <<EOT

events {}

http {
  server {

    listen              443 ssl;
    server_name         _;
    ssl_certificate     /etc/ssl/self_signed.crt;
    ssl_certificate_key /etc/ssl/self_signed.key;

    location / {
      proxy_pass https://nomad-ws;
      proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
      proxy_ssl_certificate     /etc/ssl/tls_proxy.crt;
      proxy_ssl_certificate_key /etc/ssl/tls_proxy.key;

      # Nomad blocking queries will remain open for a default of 5 minutes.
      # Increase the proxy timeout to accommodate this timeout with an
      # additional grace period.
      proxy_read_timeout 310s;

      # Nomad log streaming uses streaming HTTP requests. In order to
      # synchronously stream logs from Nomad to NGINX to the browser
      # proxy buffering needs to be turned off.
      proxy_buffering off;

      # The Upgrade and Connection headers are used to establish
      # a WebSockets connection.
      proxy_set_header Upgrade $http_upgrade;
      proxy_set_header Connection "upgrade";

      # The default Origin header will be the proxy address, which
      # will be rejected by Nomad. It must be rewritten to be the
      # host address instead.
      proxy_set_header Origin "${scheme}://${proxy_host}";
    }
  }

  # WebSockets are stateful connections but we're deploying only one proxy
  # and proxying to the local Nomad client. That client will stream RPCs
  # from the server. But we've left ip_hash here in case someone comes
  # along and copy-and-pastes this configuration elsewhere without reading
  # the Learn Guide.
  upstream nomad-ws {
    ip_hash;
    server {{ env "attr.unique.network.ip-address" }}:4646;
  }
}

EOT
      }


    }
  }
}
