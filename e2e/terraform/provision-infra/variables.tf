# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "name" {
  description = "Used to name various infrastructure components"
  default     = "nomad-e2e"
}

variable "region" {
  description = "The AWS region to deploy to."
  default     = "us-east-1"
}

variable "availability_zone" {
  description = "The AWS availability zone to deploy to."
  default     = "us-east-1b"
}

variable "instance_type" {
  description = "The AWS instance type to use for both clients and servers."
  default     = "t3a.medium"
}

variable "instance_architecture" {
  description = "The architecture for the AWS instance type to use for both clients and servers."
  default     = "amd64"
}

variable "server_count" {
  description = "The number of servers to provision."
  default     = "3"
}

variable "client_count_linux" {
  description = "The number of Ubuntu clients to provision."
  default     = "4"
}

variable "client_count_windows_2016_amd64" {
  description = "The number of windows 2016 clients to provision."
  default     = "0"
}

variable "restrict_ingress_cidrblock" {
  description = "Restrict ingress traffic to cluster to invoker ip address"
  type        = bool
  default     = true
}

# ----------------------------------------
# The specific version of Nomad deployed will default to whichever one of
# nomad_sha, nomad_version, or nomad_local_binary is set

variable "nomad_local_binary" {
  description = "The path to a local binary to provision"
  default     = ""
}

variable "nomad_region" {
  description = "The name of the Nomad region."
  default     = "e2e"
}

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

variable "volumes" {
  type        = bool
  description = "Include external EFS volumes (for CSI)"
  default     = true
}


variable "hcp_vault_cluster_id" {
  description = "The ID of the HCP Vault cluster"
  type        = string
  default     = "nomad-e2e-shared-hcp-vault"
}

variable "hcp_vault_namespace" {
  description = "The namespace where the HCP Vault cluster policy works"
  type        = string
  default     = "admin"
}

variable "aws_kms_alias" {
  description = "The alias for the AWS KMS key ID"
  type        = string
  default     = "kms-nomad-keyring"
}

# ----------------------------------------
# If you want to deploy multiple versions you can use these variables to
# provide a list of builds to override the values of nomad_sha, nomad_version,
# or nomad_local_binary. Most of the time you can ignore these variables!

variable "nomad_local_binary_server" {
  description = "A list of nomad local binary paths to deploy to servers, to override nomad_local_binary"
  type        = list(string)
  default     = []
}

variable "nomad_local_binary_client_ubuntu_jammy_amd64" {
  description = "A list of nomad local binary paths to deploy to Ubuntu Jammy clients, to override nomad_local_binary"
  type        = list(string)
  default     = []
}

variable "nomad_local_binary_client_windows_2016_amd64" {
  description = "A list of nomad local binary paths to deploy to Windows 2016 clients, to override nomad_local_binary"
  type        = list(string)
  default     = []
}