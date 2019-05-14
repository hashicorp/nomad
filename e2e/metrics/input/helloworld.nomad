job "hello" {
  datacenters = ["dc1"]

  update {
    max_parallel = 1
    min_healthy_time = "15s"
    auto_revert = true
  }

  group "hello" {

    count = 3

    task "hello" {
      driver = "raw_exec"

      config {
        command = "local/hello"
      }

      artifact {
        source = "https://nomad-community-demo.s3.amazonaws.com/hellov1"
        destination = "local/hello"
        mode = "file"
      }

      resources {
        cpu    = 500
        memory = 256
        network {
          mbits = 10
          port "web" {}
        }
      }

      service {
        name = "hello"
        tags = ["urlprefix-hello/"]
        port = "web"
        check {
          name     = "alive"
          type     = "http"
          path     = "/"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
