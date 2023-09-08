# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

type            = "csi"
id              = "testvol"
name            = "test_volume"
external_id     = "56as4da-as524d-asd9-asd8-asdasd52555"
access_mode     = "single-node-writer"
attachment_mode = "file-system"
plugin_id       = "cinder-csi"
mount_options {
  fs_type = "ext4"
}
