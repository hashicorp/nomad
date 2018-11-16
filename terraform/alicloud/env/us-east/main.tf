variable "access_key" {}
variable "secret_key" {}

variable "name" {
  description = "Used to name various infrastructure components"
  default     = "hashistack"
}

variable "region" {
  description = "The Alicloud region to deploy to."
  default     = "us-east-1"
}

variable "image_id" {}

variable "instance_type" {}


variable "server_count" {
  description = "The number of servers to provision."
  default     = "3"
}

variable "client_count" {
  description = "The number of clients to provision."
  default     = "4"
}

locals {
  "retry_join" = "provider=aliyun region=${var.region} tag_key=ConsulAutoJoin tag_value=auto-join access_key_id=${var.access_key} access_key_secret=${var.secret_key}"
}

variable "nomad_binary" {
  description = "Used to replace the machine image installed Nomad binary."
  default     = "https://releases.hashicorp.com/nomad/0.8.6/nomad_0.8.6_linux_amd64.zip"
}

variable "nic_type" {
  default = "intranet"
}

variable "internet_charge_type" {
  default = "PayByTraffic"
}

variable "internet_max_bandwidth_out" {
  default = 5
}

variable "disk_category" {
  default = "cloud_efficiency"
}

variable "disk_size" {
  default = "40"
}

variable "key_name" {
  default = "key-pair-from-terraform"
}

variable "private_key_file" {
  default = "alicloud_ssh_key.pem"
}

provider "alicloud" {
  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
  region = "${var.region}"
}

variable "vpc_cidr" {
  default = "172.16.0.0/12"
}
variable "vswitch_cidr" {
  default = "172.16.0.0/21"
}
variable "zone" {
  default = "us-east-1a"
}

module "hashistack" {
  source = "../../modules/hashistack"

  name                       = "${var.name}"
  region                     = "${var.region}"
  image_id                   = "${var.image_id}"
  instance_type              = "${var.instance_type}"
  server_count               = "${var.server_count}"
  client_count               = "${var.client_count}"
  retry_join                 = "${local.retry_join}"
  nomad_binary               = "${var.nomad_binary}"
  nic_type                   = "${var.nic_type}"
  internet_charge_type       = "${var.internet_charge_type}"
  internet_max_bandwidth_out = "${var.internet_max_bandwidth_out}"
  disk_category              = "${var.disk_category}"
  disk_size                  = "${var.disk_size}"
  key_name                   = "${var.key_name}"
  private_key_file           = "${var.private_key_file}"
  vpc_cidr                   = "${var.vpc_cidr}"
  vswitch_cidr               = "${var.vswitch_cidr}"
  zone                       = "${var.zone}"
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

The Nomad UI can be accessed at http://PUBLIC_IP:4646/ui.
The Consul UI can be accessed at http://PUBLIC_IP:8500/ui.

CONFIGURATION
}
