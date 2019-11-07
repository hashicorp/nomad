job "completed_leader" {
  type        = "batch"
  datacenters = ["dc1"]

  group "completed_leader" {
    restart {
      attempts = 0
    }

    # Only the task named the same as the job has its events tested.
    task "completed_leader" {
      driver = "raw_exec"

      config {
        command = "sleep"
        args    = ["1000"]
      }
    }

    task "leader" {
      leader = true
      driver = "raw_exec"

      config {
        command = "sleep"
        args    = ["1"]
      }
    }
  }
}
