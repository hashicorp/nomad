# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# this variable is not used but required by runner
variable "alloc_count" {
  type    = number
  default = 2
}

job "countdash" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "api" {
    network {
      mode = "bridge"
    }

    service {
      name = "count-api"
      port = "9001"

      check {
        type     = "http"
        path     = "/health"
        expose   = true
        interval = "3s"
        timeout  = "1s"

        check_restart {
          limit = 0 # don't restart on failure
        }
      }

      connect {
        sidecar_service {
          proxy {
            transparent_proxy {}
          }
        }
      }
    }

    task "web" {
      driver = "docker"

      config {
        image          = "hashicorpdev/counter-api:v3"
        auth_soft_fail = true
      }
    }
  }

  group "dashboard" {
    network {
      mode = "bridge"

      port "http" {
        # TODO: for some reason without a static port the health check never
        # succeeds, even though we have expose=true on the check
        static = 9002
        to     = 9002
      }
    }

    service {
      name = "count-dashboard"
      port = "9002"

      # this check will fail if connectivity between the dashboard and the API
      # fails, and restart the task. we poll frequently but also allow it to
      # fail temporarily so we can account for allocations being rescheduled
      # during tests
      check {
        type     = "http"
        path     = "/health"
        expose   = true
        task     = "dashboard"
        interval = "3s"
        timeout  = "1s"

        # note it seems to take an extremely long time for this API to return ok
        check_restart {
          limit = 30
        }
      }

      connect {
        sidecar_service {
          proxy {
            transparent_proxy {}
          }
        }
      }
    }

    # note: this is not the usual countdash frontend because that only sets the
    # health check that tests the backend as healthy once a browser connection
    # has been made. So serve a reverse proxy to the count API instead.
    task "dashboard" {
      driver = "docker"

      env {
        COUNTING_SERVICE_URL = "http://count-api.virtual.consul"
      }

      config {
        image          = "nginx:latest"
        command        = "nginx"
        args           = ["-c", "/local/default.conf"]
        auth_soft_fail = true
      }

      template {
        destination = "local/default.conf"
        data        = <<EOT
daemon off;
worker_processes  1;
user www-data;
error_log /var/log/error.log info;

events {
  use epoll;
  worker_connections 128;
}

http {
  include /etc/nginx/mime.types;
  charset utf-8;
  access_log /var/log/access.log  combined;
  server {
    listen 9002;
    location / {
      proxy_pass http://count-api.virtual.consul;
    }
  }
}
EOT

      }

      # restart only once because we're using the service for this task to
      # detect tproxy connectivity failures in this test
      restart {
        delay    = "5s"
        attempts = 1
        mode     = "fail"
      }
    }

  }
}
