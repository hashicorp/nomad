# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

job "job1" {
  type        = "service"
  datacenters = ["dc1"]
  group "group1" {
    count = 1
    task "task1" {
      driver = "exec"
      config {
        command = "/bin/sleep"
      }
      resources {
        cpu    = 1000
        memory = 512
      }
    }
  }
}
