job "sleep" {
  datacenters = ["dc1"]

  group "sleep" {
    task "sleep" {
      driver = "raw_exec"

      config {
        command = "sleep"
        args = ["10000"]
      }

      resources {
        cpu    = 100
        memory = 32
      }
    }
  }
}

