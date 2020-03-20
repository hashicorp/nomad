job "foo" {
  group "example" {
    task "server" {
      resources {
        network {
          mbits = 200
          port "http" {}
          port "https" {}
          port "lb" {
            static = "8889"
          }
          dynamic_port_range {
              min = 4000
              max = 5000
          }
        }
      }
    }
  }
}