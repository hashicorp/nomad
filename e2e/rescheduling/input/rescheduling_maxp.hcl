job "demo2" {
  datacenters = ["dc1"]
  type        = "service"

  group "t2" {
    count = 3

    task "t2" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 5000"]
      }
    }

    update {
      max_parallel     = 1
      min_healthy_time = "5s"
      healthy_deadline = "10m"
      auto_revert      = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    reschedule {
      unlimited = "true"

      #  attempts  = 0
    }
  }
}
