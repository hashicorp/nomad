# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "example" {
  group "group" {
    task "task" {
      hvs {
        org_id = "org1"
        proj_id = "proj1"
        wip_name = "my_wip"
      }
    }
  }
}
