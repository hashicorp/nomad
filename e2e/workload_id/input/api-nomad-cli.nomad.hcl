# Copyright (c) HashiCorp, Inc.
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
        // "|| true" because failure to get a var makes nomad cli exit 1,
        // but for this test, "Variable not found" actually indicates successful
        // API connection.
        args = ["-xc", "echo $NOMAD_ADDR; nomad var get nothing || true"]
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
