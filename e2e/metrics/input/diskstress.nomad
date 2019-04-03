job "diskstress" {
  datacenters = ["dc1", "dc2"]
  type = "batch"
  group "diskstress" {
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
    task "diskstress" {
      driver = "docker"

      config {
        image = "progrium/stress"
        args = [
          "-d", "2",
          "-t", "30"
        ]

      }
      resources {
        cpu    = 4096
        memory = 256
      }
    }
  }
}