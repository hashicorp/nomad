job "parse-ports" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    count = 1

    network {
      port "static" {
        static = 9000
      }

      port "dynamic" {}
    }

    task "parse-port" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "nc"
        args    = ["-ll", "-p", "1234", "-e", "/bin/cat"]
        ports   = ["static", "dynamic"]
      }
    }
  }
}
