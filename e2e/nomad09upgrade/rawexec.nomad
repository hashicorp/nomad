job "sleep" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "sleep" {
    task "sleep" {
      driver = "raw_exec"

      config {
        command = "sleep"
        args    = ["10000"]
      }

      resources {
        cpu    = 100
        memory = 32
      }
    }
  }
}
