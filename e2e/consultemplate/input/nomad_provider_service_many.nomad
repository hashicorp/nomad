job "nomad_provider_service_many" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "caching" {

    count = 4

    service {
      name     = "redis"
      provider = "nomad"
    }

    task "test" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
      resources {
        cpu = 10
        memory = 10
      }
    }
  }
}
