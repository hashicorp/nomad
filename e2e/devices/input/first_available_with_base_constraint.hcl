# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

# Test for first_available with base constraints.
# The device block has a base constraint that all options must satisfy,
# plus each first_available option can have additional constraints.

job "first-available-base" {
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
          # Base constraint applied to all first_available options
          constraint {
            attribute = "${device.attr.cool-attribute}"
            value     = "attribute-wearing-sunglasses"
          }

          first_available {
            count = 2
            constraint {
              attribute = "${device.attr.package}"
              value     = "premium"
            }
          }
          first_available {
            count = 1
            constraint {
              attribute = "${device.attr.package}"
              value     = "standard"
            }
          }
        }
      }
    }
  }
}
