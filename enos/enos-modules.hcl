// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

module "build_artifactory" {
  source = "./modules/fetch_artifactory"
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

module "upgrade_clients" {
  source = "./modules/upgrade_clients"
}
