# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

provider "aws" {
  region = var.region
}

module "provision-infra" {
  source = "./provision-infra"

  server_count                           = var.server_count
  client_count_linux                     = var.client_count_linux
  client_count_windows_2016              = var.client_count_windows_2016
  nomad_local_binary_server              = var.nomad_local_binary_server
  nomad_local_binary                     = var.nomad_local_binary
  nomad_local_binary_client_ubuntu_jammy = var.nomad_local_binary_client_ubuntu_jammy
  nomad_local_binary_client_windows_2016 = var.nomad_local_binary_client_windows_2016
  nomad_license                          = var.nomad_license
  consul_license                         = var.consul_license
  nomad_region                           = var.nomad_region
  instance_arch                          = var.instance_arch
  name                                   = var.name
}
