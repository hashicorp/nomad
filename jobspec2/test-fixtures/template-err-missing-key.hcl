# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "example" {
  group "group" {
    task "task" {
      template {
        error_on_missing_key = true
      }
    }
  }
}
