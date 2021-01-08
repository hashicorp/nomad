job "test" {
  datacenters = ["dc1"]
  type        = "batch"

  periodic {
    cron             = "* * * * *"
    prohibit_overlap = true
  }

  group "group" {
    task "task" {
      driver = "docker"

      config {
        image   = "alpine:latest"
        command = "ls"
      }
    }
  }
}

