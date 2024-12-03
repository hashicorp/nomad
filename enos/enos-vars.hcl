# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Variables for the fetch_artifactory module
variable "artifactory_username" {
  type        = string
  description = "The username to use when connecting to artifactory"
  default     = null
}

variable "artifactory_token" {
  type        = string
  description = "The token to use when connecting to artifactory"
  default     = null
  sensitive   = true
}

variable "product_version" {
  description = "The version of Nomad we are testing"
  type        = string
  default     = null
}

variable "binary_local_path" {
  description = "The path to donwload and unzip the binary"
  type        = string
}
