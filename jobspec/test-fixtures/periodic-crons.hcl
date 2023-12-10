# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "foo" {
  periodic {
    crons = [
      "*/5 * * *",
      "*/7 * * *"
    ]
    prohibit_overlap = true
    time_zone        = "Europe/Minsk"
  }
}
