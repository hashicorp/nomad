# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1

# Test for device constraint that cannot be satisfied.
# The job should fail to schedule because no device matches the constraint.

job "device-constraint-nomatch" {
  type = "batch"

  group "test" {
    count = 1

    task "sleep" {
      driver = "mock_driver"

      config {
        run_for = "30s"
      }

      resources {
        cpu    = 10
        memory = 64

        device "nomad/file/mock" {
          count = 1

          constraint {
            attribute = "${device.attr.cool-attribute}"
            value     = "impossible-value-that-will-never-match"
          }
        }
      }
    }
  }
}
