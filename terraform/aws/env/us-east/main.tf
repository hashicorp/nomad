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

variable "ami" {}

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
  type        = "map"

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

provider "aws" {
  region = "${var.region}"
}

module "hashistack" {
  source = "../../modules/hashistack"

  name                   = "${var.name}"
  region                 = "${var.region}"
  ami                    = "${var.ami}"
  server_instance_type   = "${var.server_instance_type}"
  client_instance_type   = "${var.client_instance_type}"
  key_name               = "${var.key_name}"
  server_count           = "${var.server_count}"
  client_count           = "${var.client_count}"
  retry_join             = "${var.retry_join}"
  nomad_binary           = "${var.nomad_binary}"
  root_block_device_size = "${var.root_block_device_size}"
  whitelist_ip           = "${var.whitelist_ip}"
}

output "IP_Addresses" {
  value = <<CONFIGURATION

Client public IPs: ${join(", ", module.hashistack.client_public_ips)}
Server public IPs: ${join(", ", module.hashistack.server_public_ips)}

To connect, add your private key and SSH into any client or server with
`ssh ubuntu@PUBLIC_IP`. You can test the integrity of the cluster by running:

  $ consul members
  $ nomad server members
  $ nomad node status

If you see an error message like the following when running any of the above
commands, it usually indicates that the configuration script has not finished
executing:

"Error querying servers: Get http://127.0.0.1:4646/v1/agent/members: dial tcp
127.0.0.1:4646: getsockopt: connection refused"

Simply wait a few seconds and rerun the command if this occurs.

The Nomad UI can be accessed at http://${module.hashistack.server_lb_ip}:4646/ui.
The Consul UI can be accessed at http://${module.hashistack.server_lb_ip}:8500/ui.

Set the following for access from the Nomad CLI:

  export NOMAD_ADDR=http://${module.hashistack.server_lb_ip}:4646

CONFIGURATION
}
