# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

type        = "csi"
id          = "efsTestVolume"
name        = "IDEMPOTENCY_TOKEN"
external_id = "EXTERNAL_ID"
plugin_id   = "aws-efs0"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}
