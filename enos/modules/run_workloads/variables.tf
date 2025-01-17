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

variable "workloads" {
  description = "A map of workloads to provision"
  type = map(object({
    template    = string
    alloc_count = number
  }))
  default = {
    service_raw_exec = { template = "templates/raw-exec-service.nomad.hcl.tpl", alloc_count = 3 }
    service_docker   = { template = "templates/docker-service.nomad.hcl.tpl", alloc_count = 3 }
  }
}
