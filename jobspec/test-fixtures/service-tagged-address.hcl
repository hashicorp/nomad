# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


job "service_tagged_address" {
  type = "service"

  group "group" {
    service {
      name = "service1"
      tagged_addresses {
        public_wan = "1.2.3.4"
      }
    }
  }
}
