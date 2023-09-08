# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "group_service_eto" {
  group "group" {
    service {
      name                = "example"
      enable_tag_override = true
    }
  }
}
