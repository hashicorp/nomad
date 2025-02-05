# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

terraform {
  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
    }
  }
}

resource "random_pet" "upgrade" {
}

resource "enos_local_exec" "wait_for_nomad_api" {
  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token

  }

  scripts = [abspath("${path.module}/scripts/wait_for_nomad_api.sh")]
}

////////////////////////////////////////////////////////////////////////////////
//                    Upgrading the first server
////////////////////////////////////////////////////////////////////////////////
// Taking a snapshot forces the cluster to store a new snapshot that will be 
// used to restore the cluster after the restart, because it will be the most 
// recent available, the resulting file wont be used..
resource "enos_local_exec" "take_first_cluster_snapshot" {
  depends_on = [enos_local_exec.wait_for_nomad_api]

  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
  }

  inline = [
    "nomad operator snapshot save ${random_pet.upgrade.id}-0.snap",
  ]
}

module upgrade_first_server {
  depends_on = [enos_local_exec.take_first_cluster_snapshot]

  source = "../upgrade_instance"

  nomad_addr                 = var.nomad_addr
  ca_file                    = var.ca_file
  cert_file                  = var.cert_file
  key_file                   = var.key_file
  nomad_token                = var.nomad_token
  platform                   = var.platform
  server_address             = var.servers[0]
  nomad_local_upgrade_binary = var.nomad_upgraded_binary
  ssh_key_path               = var.ssh_key_path
}

// This script calls `nomad server members` which returns an error if there 
// is no leader.
resource "enos_local_exec" "first_leader_verification" {
  depends_on = [module.upgrade_first_server]

  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
  }

  scripts = [abspath("${path.module}/scripts/wait_for_nomad_api.sh")]
}

////////////////////////////////////////////////////////////////////////////////
//                    Upgrading the second server
////////////////////////////////////////////////////////////////////////////////
// Taking a snapshot forces the cluster to store a new snapshot that will be 
// used to restore the cluster after the restart, because it will be the most 
// recent available, the resulting file wont be used..
resource "enos_local_exec" "take_second_cluster_snapshot" {
  depends_on = [enos_local_exec.first_leader_verification]

  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
  }

  inline = [
    "nomad operator snapshot save ${random_pet.upgrade.id}-1.snap",
  ]
}

module upgrade_second_server {
  depends_on = [enos_local_exec.take_second_cluster_snapshot]

  source = "../upgrade_instance"

  nomad_addr                 = var.nomad_addr
  ca_file                    = var.ca_file
  cert_file                  = var.cert_file
  key_file                   = var.key_file
  nomad_token                = var.nomad_token
  platform                   = var.platform
  server_address             = var.servers[1]
  nomad_local_upgrade_binary = var.nomad_upgraded_binary
  ssh_key_path               = var.ssh_key_path
}

// This script calls `nomad server members` which returns an error if there 
// is no leader.
resource "enos_local_exec" "second_leader_verification" {
  depends_on = [module.upgrade_second_server]

  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
  }

  scripts = [abspath("${path.module}/scripts/wait_for_nomad_api.sh")]
}

////////////////////////////////////////////////////////////////////////////////
//                    Upgrading the third server
////////////////////////////////////////////////////////////////////////////////
// Taking a snapshot forces the cluster to store a new snapshot that will be 
// used to restore the cluster after the restart, because it will be the most 
// recent available, the resulting file wont be used.
resource "enos_local_exec" "take_third_cluster_snapshot" {
  depends_on = [enos_local_exec.second_leader_verification]

  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
  }

  inline = [
    "nomad operator snapshot save ${random_pet.upgrade.id}-2.snap",
  ]
}

module upgrade_third_server {
  depends_on = [enos_local_exec.take_third_cluster_snapshot]

  source = "../upgrade_instance"

  nomad_addr                 = var.nomad_addr
  ca_file                    = var.ca_file
  cert_file                  = var.cert_file
  key_file                   = var.key_file
  nomad_token                = var.nomad_token
  platform                   = var.platform
  server_address             = var.servers[2]
  nomad_local_upgrade_binary = var.nomad_upgraded_binary
  ssh_key_path               = var.ssh_key_path
}

// This script calls `nomad server members` which returns an error if there 
// is no leader.
resource "enos_local_exec" "third_leader_verification" {
  depends_on = [module.upgrade_third_server]

  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
  }

  scripts = [abspath("${path.module}/scripts/wait_for_nomad_api.sh")]
}
