# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "location" {
  description = "The Azure location to deploy to."
  default     = "East US"
}

variable "image_id" {}

variable "vm_size" {
  description = "The Azure VM size to use for both clients and servers."
  default     = "Standard_DS1_v2"
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
}

terraform {
  required_version = ">= 0.10.1"
}

provider "azurerm" {}

module "hashistack" {
  source = "../../modules/hashistack"

  location     = "${var.location}"
  image_id     = "${var.image_id}"
  vm_size      = "${var.vm_size}"
  server_count = "${var.server_count}"
  client_count = "${var.client_count}"
  retry_join   = "${var.retry_join}"
}

output "IP_Addresses" {
  value = <<CONFIGURATION

Client public IPs: ${join(", ", module.hashistack.client_public_ips)}
Server public IPs: ${join(", ", module.hashistack.server_public_ips)}

To connect, add your private key and SSH into any client or server with
`ssh ubuntu@PUBLIC_IP`. You can test the integrity of the cluster by running:

  $ consul members
  $ nomad server-members
  $ nomad node-status

If you see an error message like the following when running any of the above
commands, it usually indicates that the configuration script has not finished
executing:

"Error querying servers: Get http://127.0.0.1:4646/v1/agent/members: dial tcp
127.0.0.1:4646: getsockopt: connection refused"

Simply wait a few seconds and rerun the command if this occurs.

The Nomad UI can be accessed at http://PUBLIC_IP:4646/ui.
The Consul UI can be accessed at http://PUBLIC_IP:8500/ui.

CONFIGURATION
}
