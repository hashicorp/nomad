# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

client {
  meta {
    "rack" = "r1"
  }

  host_volume "shared_data" {
    path = "/srv/data"
  }
}
