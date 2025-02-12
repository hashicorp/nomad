# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "nomad_addr" {
  description = "The Nomad API HTTP address of the instance being upgraded."
  type        = string
  default     = "http://localhost:4646"
}

variable "nomad_token" {
  description = "The Secret ID of an ACL token to make requests with, for ACL-enabled clusters."
  type        = string
}

variable "platform" {
  description = "Operating system of the instance to upgrade"
  type        = string
  default     = "linux"
}

variable "instance_address" {
  description = "Public IP address of the instance that will be updated"
  type        = string
}

variable "ssh_key_path" {
  description = "Path to the ssh private key that can be used to connect to the instance where the server is running"
  type        = string
}

variable "artifactory_release" {
  type = object({
    username = string
    token    = string
    url      = string
    sha256   = string
  })
  description = "The Artifactory release information to install Nomad artifacts from Artifactory"
  default     = null
}

variable "tls" {
  type = object({
    ca_file   = string
    cert_file = string
    key_file  = string
  })
  description = "Paths to tls keys and certificates for Nomad CLI"
  default     = null
}
