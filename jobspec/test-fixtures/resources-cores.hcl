job "cores-test" {
  group "group" {
    count = 5

    task "task" {
      driver = "docker"

      resources {
        cores  = 4
        memory = 128
      }
    }
  }
}
