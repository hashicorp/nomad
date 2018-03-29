job "test4" {
  datacenters = ["dc1"]
  type        = "service"

  group "t4" {
    count = 3

    task "t4" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 5000"]
      }
    }

    update {
      max_parallel     = 1
      min_healthy_time = "10s"
      auto_revert      = false
    }

    restart {
      attempts = 0
      delay    = "0s"
      mode     = "fail"
    }

    reschedule {
      attempts  = 3
      interval  = "5m"
      unlimited = false
    }
  }
}
