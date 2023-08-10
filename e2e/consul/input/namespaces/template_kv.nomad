# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "template_kv" {
  datacenters = ["dc1"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group-b" {

    consul {
      namespace = "banana"
    }

    task "task-b" {
      driver = "raw_exec"

      config {
        command = "cat"
        args    = ["local/a.txt"]
      }

      template {
        data        = "value: {{ key \"ns-kv-example\" }}"
        destination = "local/a.txt"
      }
    }
  }

  group "group-z" {

    # no consul namespace set

    task "task-z" {
      driver = "raw_exec"

      config {
        command = "cat"
        args    = ["local/a.txt"]
      }

      template {
        data        = "value: {{ key \"ns-kv-example\" }}"
        destination = "local/a.txt"
      }
    }
  }
}