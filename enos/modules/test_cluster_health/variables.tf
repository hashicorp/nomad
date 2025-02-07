# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

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
}

variable "server_count" {
  description = "The expected number of servers."
  type        = number
}

variable "client_count" {
  description = "The expected number of Ubuntu clients."
  type        = number
}

variable "jobs_count" {
  description = "The number of jobs that should be running in the cluster"
  type        = number
}

variable "alloc_count" {
  description = "Number of allocation that should be running in the cluster"
  type        = number
}

variable "clients_version" {
  description = "Binary version running on the clients"
  type        = string
}

variable "servers_version" {
  description = "Binary version running on the servers"
  type        = string
}

variable "servers" {
  description = "List of public IP address of the nomad servers"
  type        = list
}
