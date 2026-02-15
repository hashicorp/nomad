# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

# Test for first_available when no options can be satisfied.
# All options have impossible constraints, so the job should fail to schedule.

job "device-first-available-nomatch" {
  type = "batch"

  group "test" {
    count = 1

    task "sleep" {
      driver = "raw_exec"

      config {
        command = "sleep"
        args    = ["30"]
      }

      resources {
        cpu    = 10
        memory = 64

        device "nomad/file/mock" {
          first_available {
            count = 100
            constraint {
              attribute = "${device.attr.nonexistent1}"
              value     = "impossible1"
            }
          }
          first_available {
            count = 100
            constraint {
              attribute = "${device.attr.nonexistent2}"
              value     = "impossible2"
            }
          }
        }
      }
    }
  }
}
