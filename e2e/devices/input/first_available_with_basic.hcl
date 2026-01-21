# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

# Test that the SECOND option is selected when the first cannot be satisfied.
# Option 1: 1 device with impossible constraint (should fail)
# Option 2: 2 devices with no constraints (should be selected)
#
# We verify by checking that exactly 2 devices were allocated.

job "device-first-available-second" {
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
          # First option: impossible constraint (should fail)
          first_available {
            count = 1
            constraint {
              attribute = "${device.attr.impossible_attr}"
              value     = "impossible_value"
            }
          }
          # Second option: request 2 devices (should be selected)
          first_available {
            count = 2
          }
          # Second option: request 3 devices (should not be selected)
          first_available {
            count = 3
          }
        }
      }
    }
  }
}
