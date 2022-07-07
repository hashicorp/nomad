variable "token" {
  type    = string
  default = "411b5a75-333a-45d8-9435-8ea23c9cf63d"
}

locals {
  config-template = <<-EOF
    authtoken ${var.token}
  EOF
}

job "lost_template_with_vars" {
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
        data        = "${local.config-template}"
        destination = "${NOMAD_ALLOC_DIR}/data/password.txt"
        change_mode = "restart"
      }
    }
  }
}
