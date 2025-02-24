# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "artifactory_credentials" {
  description = "Credentials for connecting to Artifactory"

  type = object({
    username = string
    token    = string
  })

  sensitive = true
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

variable "binary_config" {
  type = object({
    edition         = string
    os              = string
    product_version = string
    arch            = string
  })

  description = "Configuration for fetching the binary"

  default = {
    edition         = "ce"
    os              = "linux"
    product_version = null
    arch            = null
  }

  validation {
    condition     = contains(["ent", "ce"], var.binary_config.edition)
    error_message = "Edition must be one of 'ent' or 'ce'."
  }
}

variable "download_binary" {
  description = "Used to control if the artifact should be downloaded to the local instance or not"
  default     = true
}

variable "download_binary_path" {
  description = "A directory path on the local instance where the artifacts will be installed (requires download_binary is true)"
  type        = string
  default     = "/home/ubuntu/nomad"
}
