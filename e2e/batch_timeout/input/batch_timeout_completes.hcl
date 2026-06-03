# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1

# batch_timeout_completes: a batch job whose task finishes almost immediately.
# With a 30s max_run_duration the job should complete normally.

job "batch-timeout-completes" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    # generous timeout
    max_run_duration = "30s"

    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    task "echo" {
      driver = "raw_exec"

      config {
        command = "echo"
        args    = ["hello from batch-timeout-completes"]
      }

      resources {
        cpu    = 10
        memory = 10
      }
    }
  }
}
