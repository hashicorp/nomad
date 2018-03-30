job "demo2" {
  datacenters = ["dc1"]
  type        = "service"

  group "t2" {
    count = 3

    task "t2" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 7000"]
      }
    }

    update {
      max_parallel     = 1
      min_healthy_time = "1s"
      healthy_deadline = "1m"
      auto_revert      = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    reschedule {
      unlimited = "true"
    }
  }
}
