# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "name" {
  description = "Used to name various infrastructure components"
}

variable "whitelist_ip" {
  description = "IP to whitelist for the security groups (set 0.0.0.0/0 for world)"
}

variable "region" {
  description = "The AWS region to deploy to."
  default     = "us-east-1"
}

variable "ami" {
}

variable "server_instance_type" {
  description = "The AWS instance type to use for servers."
  default     = "t2.medium"
}

variable "client_instance_type" {
  description = "The AWS instance type to use for clients."
  default     = "t2.medium"
}

variable "root_block_device_size" {
  description = "The volume size of the root block device."
  default     = 16
}

variable "key_name" {
  description = "Name of the SSH key used to provision EC2 instances."
}

variable "server_count" {
  description = "The number of servers to provision."
  default     = "3"
}

variable "client_count" {
  description = "The number of clients to provision."
  default     = "4"
}

variable "retry_join" {
  description = "Used by Consul to automatically form a cluster."
  type        = map(string)

  default = {
    provider  = "aws"
    tag_key   = "ConsulAutoJoin"
    tag_value = "auto-join"
  }
}

variable "nomad_binary" {
  description = "Used to replace the machine image installed Nomad binary."
  default     = "none"
}
