# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "foo" {
  periodic {
    cron             = "*/5 * * *"
    prohibit_overlap = true
    time_zone        = "Europe/Minsk"
  }
}
