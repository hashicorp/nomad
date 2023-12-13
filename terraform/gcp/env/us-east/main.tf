# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

terraform {
  required_version = ">= 0.12.0"
}

variable "project" {
  type        = string
  default     = ""
  description = "The Google Cloud Platform project to deploy the Nomad cluster in."
}

variable "credentials" {
  type        = string
  default     = ""
  description = "The path to the Google Cloud Platform credentials file (in JSON format) to use."
}

variable "region" {
  type        = string
  default     = "us-east1"
  description = "The GCP region to deploy resources in."
}

variable "vm_disk_size_gb" {
  description = "The GCP disk size to use both clients and servers."
  default     = "50"
}

variable "server_count" {
  description = "The number of servers to provision."
  default     = "3"
}

variable "client_count" {
  description = "The number of clients to provision."
  default     = "4"
}

provider "google" {
  project     = var.project
  credentials = file(var.credentials)
}

module "hashistack" {
  source              = "../../modules/hashistack"
  project             = var.project
  credentials         = var.credentials
  server_disk_size_gb = var.vm_disk_size_gb
  server_count        = var.server_count
  client_count        = var.client_count
}
