job "lost_template" {
  datacenters = ["dc1"]

  group "group" {
    count = 2

    max_client_disconnect = "1h"

    constraint {
      attribute = "${attr.kernel.name}"
      value     = "linux"
    }

    constraint {
      operator = "distinct_hosts"
      value    = "true"
    }

    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "httpd"
        args    = ["-v", "-f", "-p", "8001", "-h", "/var/www"]
      }

      resources {
        cpu    = 128
        memory = 128
      }

      template {
        data        = "---\nkey: {{ timestamp }}"
        destination = "local/file.yml"
      }
    }
  }
}
