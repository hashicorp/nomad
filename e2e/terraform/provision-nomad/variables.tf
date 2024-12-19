# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "nomad_region" {
  type        = string
  description = "Cluster region"
  default     = "e2e"
}

variable "nomad_local_binary" {
  type        = string
  description = "Path to local Nomad build (ex. \"/home/me/bin/nomad\")"
  default     = ""
}

variable "nomad_license" {
  type        = string
  description = "The enterprise license to use. overrides Nomad temporary license"
  default     = ""
}

variable "tls_ca_algorithm" {
  type        = string
  description = "CA private key algorithm"
  default     = "ECDSA"
}

variable "tls_ca_key" {
  type        = string
  description = "Cluster TLS CA private key"
  default     = ""
}

variable "tls_ca_cert" {
  type        = string
  description = "Cluster TLS CA cert"
  default     = ""
}

variable "arch" {
  type        = string
  description = "The architecture for this instance (ex. 'linux_amd64' or 'windows_amd64')"
  default     = "linux_amd64"
}

variable "platform" {
  type        = string
  description = "The platform for this instance (ex. 'windows' or 'linux')"
  default     = "linux"
}

variable "role" {
  type        = string
  description = "The role for this instance (ex. 'client' or 'server')"
  default     = ""
}

variable "index" {
  type        = string # note that we have string here so we can default to ""
  description = "The count of this instance for indexed configurations"
  default     = ""
}

variable "instance" {
  type = object({
    id          = string
    public_dns  = string
    public_ip   = string
    private_dns = string
    private_ip  = string
  })
}

variable "connection" {
  type = object({
    user        = string
    port        = number
    private_key = string
  })
  description = "ssh connection information for remote target"
}

variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "aws_kms_key_id" {
  type        = string
  description = "AWS KMS key ID for encrypting and decrypting the Nomad keyring"
  default     = ""
}
