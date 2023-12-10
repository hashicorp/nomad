# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "cgroup_devices" {
  type = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group1" {

    task "task1" {
      driver = "raw_exec"
      config {
        command = "/bin/sleep"
        args    = ["infinity"]
      }
      resources {
        cpu    = 50
        memory = 50
      }
    }
  }

  group "group2" {

    task "task2" {
      driver = "raw_exec"
      config {
        command = "/bin/sleep"
        args    = ["infinity"]
      }
      resources {
        cpu    = 50
        memory = 50
      }
    }
  }
}
