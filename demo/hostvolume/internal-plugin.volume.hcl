# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

name = "internal-plugin"
type = "host"
# this plugin is built into Nomad;
# it simply creates a directory.
plugin_id = "mkdir"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

