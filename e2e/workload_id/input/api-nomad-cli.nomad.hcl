# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

job "task-api-nomad-cli" {
  type = "batch"

  group "grp" {
    restart { attempts = 0 }
    reschedule { attempts = 0 }
    constraint {
      attribute = "${attr.kernel.name}"
      value     = "linux"
    }

    task "tsk" {
      driver = "raw_exec"
      config {
        command = "bash"
        args    = ["-xc", "echo $NOMAD_ADDR; nomad var get nomad/jobs/task-api-nomad-cli"]
      }
      env {
        NOMAD_ADDR = "${NOMAD_UNIX_ADDR}"
      }
      identity {   # creates unix addr
        env = true # provides NOMAD_TOKEN
      }
    }
  }
}
