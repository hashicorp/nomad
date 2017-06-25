variable "region" {
  description = "The AWS region to deploy to."
  default     = "us-east-1"
}

variable "ami" {}

variable "instance_type" {
  description = "The AWS instance type to use for both clients and servers."
  default     = "t2.medium"
}

variable "key_name" {}

variable "server_count" {
  description = "The number of servers to provision."
  default     = "3"
}

variable "client_count" {
  description = "The number of clients to provision."
  default     = "4"
}

variable "cluster_tag_value" {
  description = "Used by Consul to automatically form a cluster."
  default     = "auto-join"
}

provider "aws" {
  region = "${var.region}"
}

module "hashistack" {
  source = "../../modules/hashistack"

  region            = "${var.region}"
  ami               = "${var.ami}"
  instance_type     = "${var.instance_type}"
  key_name          = "${var.key_name}"
  server_count      = "${var.server_count}"
  client_count      = "${var.client_count}"
  cluster_tag_value = "${var.cluster_tag_value}"
}

output "primary_server_private_ips" {
  value = "${module.hashistack.primary_server_private_ips}"
}

output "primary_server_public_ips" {
  value = "${module.hashistack.primary_server_public_ips}"
}

output "client_private_ips" {
  value = "${module.hashistack.client_private_ips}"
}

output "client_public_ips" {
  value = "${module.hashistack.client_public_ips}"
}
