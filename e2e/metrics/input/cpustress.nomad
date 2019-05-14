job "cpustress" {
  datacenters = ["dc1", "dc2"]
  type = "batch"
  group "cpustress" {
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
    task "cpustress" {
      driver = "docker"

      config {
        image = "progrium/stress"
        args = [
          "-c", "4",
          "-t", "600"
        ]

      }

      resources {
        cpu    = 4096
        memory = 256
      }
    }
  }
}