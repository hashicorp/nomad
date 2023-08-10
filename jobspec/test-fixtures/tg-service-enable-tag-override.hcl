# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "group_service_eto" {
  group "group" {
    service {
      name                = "example"
      enable_tag_override = true
    }
  }
}
