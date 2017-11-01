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

  location          = "${var.location}"
  image_id          = "${var.image_id}"
  vm_size           = "${var.vm_size}"
  server_count      = "${var.server_count}"
  client_count      = "${var.client_count}"
  retry_join        = "${var.retry_join}"

}
