# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1

# batch_timeout_basic: a batch job whose sole task sleeps for a very long time.
# The max_run_duration of 5s ensures it is terminated before it finishes
# naturally, so the allocation should end in "complete" with the timeout
# description.

job "batch-timeout-basic" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    # The allocation must be killed after 5 seconds.
    max_run_duration = "5s"

    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

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
  }
}
