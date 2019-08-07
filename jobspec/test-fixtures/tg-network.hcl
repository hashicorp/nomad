job "foo" {
  datacenters = ["dc1"]
  group "bar" {
    count = 3
    network {
      mode = "bridge"
      port "http" {
        static = 80
        to = 8080
      }
    }
      
    service {
      name        = "connect-service"
      tags        = ["foo", "bar"]
      canary_tags = ["canary", "bam"]
      port        = "1234"

      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "other-service"
              local_bind_port  = 4567
            }
          }
        }
      }
    }

    task "bar" {
      driver = "raw_exec"
      config {
         command = "bash"
         args    = ["-c", "echo hi"]
      }
      resources {
        network {
            mbits = 10
        }
      }
    }
  }
}
