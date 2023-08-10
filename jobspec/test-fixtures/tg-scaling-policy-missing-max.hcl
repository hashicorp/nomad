# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "elastic" {
  group "group" {
    scaling {
      // required: max = ...
    }
  }
}
