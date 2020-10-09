job "sysbatchjob" {
  datacenters = ["dc1"]

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
        image = "bash:5"

        command = "bash"
        args    = ["-c", "ping -c 10 example.com"]
      }
    }
  }
}
