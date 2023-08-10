# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "do_token" {
  description = "API key"
}

variable "region" {
  default = "nyc1"
}

variable "volume_id" {
  default = "nomad-csi-test"
}
