# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: MPL-2.0

job "example" {
  group "group" {
    task "task" {
      template {
        error_on_missing_key = true
      }
    }
  }
}
