# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

client {
  meta {
    "rack" = "r2"
  }

  host_volume "shared_data" {
    path = "/srv/data"
  }
}
