# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

name      = "created-volume"
type      = "host"
plugin_id = "mkdir"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}