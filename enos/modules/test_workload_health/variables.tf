# Copyright IBM Corp. 2015, 2025
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

variable "client_count" {
  description = "The expected number of Ubuntu clients."
  type        = number
}

variable "jobs" {
  description = "A list of all jobs."
  type        = set(string)
}

variable "service_jobs" {
  description = "A list of all service jobs."
  type        = set(string)
}

variable "system_jobs" {
  description = "A list of all system jobs."
  type        = set(string)
}

variable "batch_jobs" {
  description = "A list of all batch jobs."
  type        = set(string)
}

variable "sysbatch_jobs" {
  description = "A list of all sysbatch jobs."
  type        = set(string)
}
