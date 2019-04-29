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
