# There can only be a single job definition per file. This job is named
# "example" so it will create a job with the ID and Name "example".

# The "job" stanza is the top-most configuration option in the job
# specification. A job is a declarative specification of tasks that Nomad
# should run. Jobs have a globally unique name, one or many task groups, which
# are themselves collections of one or many tasks.
#
# For more information and examples on the "job" stanza, please see
# the online documentation at:
#
#     https://www.nomadproject.io/docs/job-specification/job.html
#
job "example8" {

  datacenters = ["dc1"]

  type = "batch"

  group "cache" {
    # The "count" parameter specifies the number of the task groups that should
    restart {
      attempts = 0
      delay    = "30s"
      mode     = "fail"
    }

    task "miau" {
      # The "driver" parameter specifies the task driver that should be used to
      # run the task.
      driver = "raw_exec"

      config {
        command = "/system/bin/curl"
        args = ["-d", "i am a job on android", "-X", "POST", "http://ptsv2.com/t/miaaaau/post"]

      }

      resources {
        cpu    = 20
        memory = 12
        network {
          mbits = 10
          port "db" {}
        }
      }

    }
  }
}
