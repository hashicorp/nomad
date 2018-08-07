variable "name" {
  description = "Used to name various infrastructure components"
  default     = "nomad-e2e"
}

variable "region" {
  description = "The AWS region to deploy to."
  default     = "us-east-1"
}

variable "ami" {
  default = "ami-0188957e"
}

variable "instance_type" {
  description = "The AWS instance type to use for both clients and servers."
  default     = "t2.medium"
}


variable "server_count" {
  description = "The number of servers to provision."
  default     = "3"
}

variable "client_count" {
  description = "The number of clients to provision."
  default     = "3"
}

variable "retry_join" {
  description = "Used by Consul to automatically form a cluster."
  default     = "provider=aws tag_key=ConsulAutoJoin tag_value=auto-join"
}

variable "nomad_binary" {
  description = "Used to replace the machine image installed Nomad binary."
}

provider "aws" {
  region = "${var.region}"
}

module "hashistack" {
  source = "modules/hashistack"

  name          = "${var.name}"
  region        = "${var.region}"
  ami           = "${var.ami}"
  instance_type = "${var.instance_type}"
  server_count  = "${var.server_count}"
  client_count  = "${var.client_count}"
  retry_join    = "${var.retry_join}"
  nomad_binary  = "${var.nomad_binary}"
}

output "nomad_addr" {
  value = "http://${module.hashistack.server_dns}:4646"
}
