# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "name" {
  description = "Used to name various infrastructure components, must be unique per cluster"
  default     = "nomad-e2e"
}

variable "nomad_addr" {
  description = "The Nomad API HTTP address."
  type        = string
  default     = "http://localhost:4646"
}

variable "ca_file" {
  description = "A local file path to a PEM-encoded certificate authority used to verify the remote agent's certificate"
  type        = string
}

variable "cert_file" {
  description = "A local file path to a PEM-encoded certificate provided to the remote agent. If this is specified, key_file or key_pem is also required"
  type        = string
}

variable "key_file" {
  description = "A local file path to a PEM-encoded private key. This is required if cert_file or cert_pem is specified."
  type        = string
}

variable "nomad_token" {
  description = "The Secret ID of an ACL token to make requests with, for ACL-enabled clusters."
  type        = string
  sensitive   = true
}

variable "platform" {
  description = "Operative system of the instance to upgrade"
  type        = string
  default     = "linux"
}

variable "ssh_key_path" {
  description = "Path to the ssh private key that can be used to connect to the instance where the server is running"
  type        = string
}

variable "servers" {
  description = "List of public IP address of the nomad servers that will be updated"
  type        = list
}

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

variable "artifact_url" {
  type        = string
  description = "The fully qualified Artifactory item URL"
}

variable "artifact_sha" {
  type        = string
  description = "The Artifactory item SHA 256 sum"
}
