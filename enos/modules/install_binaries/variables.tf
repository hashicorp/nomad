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
  description = "The edition of the binary to search (one of ce or ent)"

  validation {
    condition     = contains(["ent", "ce"], var.edition)
    error_message = "must be one of ent or ce"
  }
}

variable "oss" {
  type        = list(string)
  description = "The operative systems the binary is needed for"
  default     = ["linux"]
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

variable "download_binaries" {
  description = "Used to control if the artifact should be downloaded to the local instance or not"
  default     = true
}

variable "download_binaries_path" {
  description = "A directory path on the local instance where the artifacts will be installed (requires download_binary is true)"
  type        = string
  default     = "/home/ubuntu/nomad"
}
