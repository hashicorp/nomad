// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

module "fetch_binaries" {
  source = "./modules/fetch_binaries"
}

module "provision_cluster" {
  source = "../e2e/terraform/provision-infra"
}

module "run_workloads" {
  source = "./modules/run_workloads"
}

module "test_cluster_health" {
  source = "./modules/test_cluster_health"
}

module "upgrade_servers" {
  source = "./modules/upgrade_servers"
}

module "upgrade_client" {
  source = "./modules/upgrade_client"
}

module "get_vault_env" {
  source = "../e2e/terraform/hcp-vault-auth"
}
  
module "drain_client" {
  source = "./modules/drain_nodes"

}
