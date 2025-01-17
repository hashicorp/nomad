# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

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

variable "artifactory_host" {
  type        = string
  description = "The artifactory host to search for Nomad artifacts"
  default     = "https://artifactory.hashicorp.engineering/artifactory"
}

variable "artifactory_repo" {
  type        = string
  description = "The artifactory repo to search for Nomad artifacts"
  default     = "hashicorp-crt-staging-local*"
}

variable "edition" {
  type        = string
  description = "The edition of the binary to search, it can be either CE or ENT"
}

variable "os" {
  type        = string
  description = "The operative system the binary is needed for"
  default     = "linux"
}

variable "product_version" {
  description = "The version of Nomad we are testing"
  type        = string
  default     = null
}

variable "arch" {
  description = "The artifactory path to search for Nomad artifacts"
  type        = string
}

variable "binary_path" {
  description = "The path to donwload and unzip the binary"
  type        = string
  default     = "/home/ubuntu/nomad"
}
