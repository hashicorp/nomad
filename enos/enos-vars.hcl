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
  description = "The version of Nomad we are starting from"
  type        = string
  default     = null
}

variable "upgrade_version" {
  description = "The version of Nomad we want to upgrade the cluster to"
  type        = string
  default     = null
}

variable "download_binary_path" {
  description = "The path to a local directory where binaries will be downloaded to provision"
}

# Variables for the provision_cluster module

variable "nomad_license" {
  type        = string
  description = "If nomad_license is set, deploy a license"
  default     = ""
}

variable "consul_license" {
  type        = string
  description = "If consul_license is set, deploy a license"
  default     = ""
}

variable "server_count" {
  description = "The number of servers to provision."
  default     = "3"
}

variable "aws_region" {
  description = "The AWS region to deploy to."
  default     = "us-east-1"
}
