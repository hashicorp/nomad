# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1

# Test for device scheduling with count, constraint, and affinity combined.

job "device-constraint-affinity" {
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
          count = 2

          constraint {
            attribute = "${device.attr.cool-attribute}"
            value     = "attribute-wearing-sunglasses"
          }

          affinity {
            attribute = "${device.attr.priority}"
            value     = "high"
            weight    = 50
          }
        }
      }
    }
  }
}
