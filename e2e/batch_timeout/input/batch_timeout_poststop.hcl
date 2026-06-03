# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1

# batch_timeout_poststop: a batch job with a poststop lifecycle task.
#
# The main task sleeps indefinitely; the max_run_duration of 5s will kill it.
# Per the feature spec, the poststop task must not be started when the
# allocation is terminated due to timeout,

job "batch-timeout-poststop" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    max_run_duration = "5s"

    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    # Main task — runs until killed by the deadline.
    task "sleep" {
      driver = "raw_exec"

      config {
        command = "sleep"
        args    = ["3600"]
      }

      resources {
        cpu    = 10
        memory = 10
      }
    }

    # poststop task must not run
    task "poststop" {
      driver = "raw_exec"

      lifecycle {
        hook    = "poststop"
        sidecar = false
      }

      config {
        command = "sleep"
        args    = ["10"]
      }

      resources {
        cpu    = 10
        memory = 10
      }
    }
  }
}
