# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

# Test for device scheduling with count and affinity.

job "device-with-affinity" {
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
          count = 1

          affinity {
            attribute = "${device.attr.cool-attribute}"
            value     = "high"
            weight    = 100
          }
        }
      }
    }
  }
}
