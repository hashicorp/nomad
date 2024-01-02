# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


job "elastic" {
  group "group" {
    scaling {
      // required: max = ...
    }
  }
}
