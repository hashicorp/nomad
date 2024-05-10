# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This manifest file describes active releases and is consumed by the backport tooling.
# It is only consumed from the default branch, so backporting changes to this file is not necessary.

schema = 1
active_versions {
  version "1.8" {
    ce_active = true
    lts       = true
  }
  version "1.7" {
    ce_active = true
  }
  version "1.6" {
    ce_active = true
  }
  version "1.5" {
    ce_active = true
  }
}
