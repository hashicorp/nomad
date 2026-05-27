# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1

# sysbatch_timeout_basic: a sysbatch job whose sole task sleeps for a very long
# time. The max_run_duration of 5s ensures it is terminated on every node before
# it finishes naturally.

job "sysbatch-timeout-basic" {
  type = "sysbatch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    max_run_duration = "5s"

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
