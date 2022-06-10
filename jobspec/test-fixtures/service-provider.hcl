job "service-provider" {
  group "group" {
    count = 5

    task "task" {
      driver = "docker"

      service {
        name     = "service-provider"
        provider = "nomad"
      }
    }
  }
}
