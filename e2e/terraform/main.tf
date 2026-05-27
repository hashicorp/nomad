# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

provider "aws" {
  region = var.region
}

module "provision-infra" {
  source = "./provision-infra"

  name                                   = var.name
  server_count                           = var.server_count
  client_count_linux                     = var.client_count_linux
  client_count_windows_2022              = var.client_count_windows_2022
  nomad_local_binary_server              = var.nomad_local_binary_server
  nomad_local_binary                     = var.nomad_local_binary
  nomad_local_binary_client_ubuntu_jammy = var.nomad_local_binary_client_ubuntu_jammy
  nomad_local_binary_client_windows_2022 = var.nomad_local_binary_client_windows_2022
  nomad_license                          = var.nomad_license
  consul_license                         = var.consul_license
  nomad_region                           = var.nomad_region
  instance_arch                          = var.instance_arch
  instance_type                          = var.instance_type
  volumes                                = var.volumes
  availability_zone                      = var.availability_zone
  aws_kms_alias                          = var.aws_kms_alias
  hcp_hvn_cidr                           = var.hcp_hvn_cidr
  hcp_vault_cluster_id                   = var.hcp_vault_cluster_id
  hcp_vault_namespace                    = var.hcp_vault_namespace
  region                                 = var.region
  restrict_ingress_cidrblock             = var.restrict_ingress_cidrblock
}
