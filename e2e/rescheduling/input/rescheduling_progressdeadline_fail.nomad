job "demo2" {

  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  type = "service"

  group "t2" {
    count = 1

    task "t2" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "exit 1"]
      }
    }

    update {
      # we want the first allocation to take a while before we give up on it,
      # so that we can check the deployment's progress deadline before and
      # after it becomes healthy
      min_healthy_time  = "10s"
      healthy_deadline  = "15s"
      progress_deadline = "20s"

      max_parallel = 1
      auto_revert  = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    reschedule {
      unlimited      = "true"
      delay_function = "constant"
      delay          = "5s"
    }
  }
}
