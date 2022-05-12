job "static-ports" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    count = 1

    network {
      port "db" {}
    }

    task "dynamic-port" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "nc"
        args    = ["-ll", "-p", "1234", "-e", "/bin/cat"]
        ports   = ["db"]
      }
    }
  }
}
