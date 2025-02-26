// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

variable "start" {}
variable "end" {}

job "test_task_schedule" {
  type = "service"

  group "group" {
    # disable deployments, because any task started outside of the schedule
    # will stay "pending" until the schedule starts it.
    update { max_parallel = 0 }

    # pausing the task should be orthogonal to this restart{} block.
    # restart{} config should only apply to the task stopping on its own,
    # as with an application error.
    restart {
      # disable restarts entirely - any application exit fails the task.
      attempts = 0
      mode     = "fail"
    }

    # don't bother rescheduling this test app
    reschedule {
      attempts = 0
    }

    task "app" {

      # feature under test
      schedule {
        cron {
          start    = var.start
          end      = var.end
          timezone = "UTC" # test "now"s are .UTC()
        }
      }

      driver = "raw_exec"
      config {
        command = "python3"
        args    = ["-c", local.app]
      }

    } # task
  }   # group
}     # job

locals {
  # this "app" just sleeps and handles signals to exit cleanly.
  app = <<EOF
import signal
import sys
import time
from datetime import datetime

def handle(sig, _frame):
    print(f'{datetime.now()} exiting: {sig=}', flush=True)
    sys.exit(0)

signal.signal(signal.SIGINT, handle)
signal.signal(signal.SIGTERM, handle)

print(f'{datetime.now()} running', flush=True)
time.sleep(10 * 60)  # 10 minutes
EOF
}
