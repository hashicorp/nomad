# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

name = "external-plugin"
type = "host"
# the executable named `example-plugin-mkfs` must be placed in the
# -host-volume-plugin-dir (config: client.host_volume_plugin_dir)
# or you will get an error creating the volume:
#  * could not place volume "external-plugin": no node meets constraints
# The default location is <data-dir>/host_volume_plugins
plugin_id    = "example-plugin-mkfs"
capacity_min = "50mb"
capacity_max = "50mb"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

parameters {
  a = "ayy"
}
