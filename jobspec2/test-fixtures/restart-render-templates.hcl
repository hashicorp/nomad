# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: MPL-2.0

job "example" {
  group "group" {
    restart {
      render_templates = true
    }
    task "foo" {
    }
    task "bar" {
      restart {
        render_templates = false
      }
    }
  }
}