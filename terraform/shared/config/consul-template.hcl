# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

vault {
  address      = "http://active.vault.service.consul:8200"
  token        = ""
  grace        = "1s"
  unwrap_token = false
  renew_token  = true
}

syslog {
  enabled  = true
  facility = "LOCAL5"
}
