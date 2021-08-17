job "sysbatchjob" {
  datacenters = ["dc1", "dc2"]

  type = "sysbatch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "sysbatch_job_group" {
    count = 1

    task "sysbatch_task" {
      driver = "docker"

      config {
        image = "busybox:1"

        command = "/bin/sh"
        args    = ["-c", "echo hi; sleep 1"]
      }
    }
  }
}
