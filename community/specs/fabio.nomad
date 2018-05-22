job "fabio" {
  datacenters = ["dc1"]

  group "fabio" {
    task "fabio" {
      driver = "docker"

      config {
        image        = "magiconair/fabio:1.5.9-go1.10.2"
        network_mode = "host"
      }

      resources {
        cpu    = 500
        memory = 256

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
