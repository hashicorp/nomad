# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# intentions.tf creates service intentions that jobs under test expect to exist
# for Consul service mesh tests

resource "consul_config_entry_service_intentions" "countdash" {
  name = "count-api"
  sources {
    name   = "count-dashboard"
    action = "allow"
    type   = "consul"
  }
}
