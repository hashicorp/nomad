# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

# Test for first_available with base constraints.
# The device block has a base constraint that all options must satisfy,
# plus each first_available option can have additional constraints.

job "device-first-available-base" {
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
          # Base constraint applied to all first_available options
          constraint {
            attribute = "${device.attr.cool-attribute}"
            value     = "attribute-wearing-sunglasses"
          }

          first_available {
            count = 2
            constraint {
              attribute = "${device.attr.type}"
              value     = "premium"
            }
          }
          first_available {
            count = 1
            constraint {
              attribute = "${device.attr.type}"
              value     = "standard"
            }
          }
        }
      }
    }
  }
}
