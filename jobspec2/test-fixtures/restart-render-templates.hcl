# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

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