# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variable "do_token" {
  description = "API key"
}

variable "region" {
  default = "nyc1"
}

variable "volume_id" {
  default = "nomad-csi-test"
}
