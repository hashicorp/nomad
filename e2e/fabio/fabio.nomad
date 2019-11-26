job "fabio" {
  datacenters = ["dc1", "dc2"]
  type        = "system"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "fabio" {
    task "fabio" {
      driver = "docker"

      config {
        image        = "fabiolb/fabio"
        network_mode = "host"
      }

      resources {
        cpu    = 100
        memory = 64

        network {
          mbits = 20

          port "lb" {
            static = 9999
          }

          port "ui" {
            static = 9998
          }
        }
      }
    }
  }
}
