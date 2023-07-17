job "namespace_default" {

  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    task "task" {

      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      resources {
        cpu    = 256
        memory = 128
      }
    }
  }
}
