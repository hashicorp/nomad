job "memstress" {
  datacenters = ["dc1", "dc2"]
  type = "batch"
  group "memstress" {
    count = 1
    restart {
      mode = "fail"
      attempts = 0
    }
    reschedule {
      attempts = 3
      interval = "10m"
      unlimited = false
    }
    task "memstress" {
      driver = "docker"

      config {
        image = "progrium/stress"
        args = [
          "-m", "2",
          "-t", "120"
        ]

      }
      resources {
        cpu    = 4096
        memory = 1024
      }
    }
  }
}