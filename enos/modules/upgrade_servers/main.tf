# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

terraform {
  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
    }
  }
}

locals {
  nomad_env = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
    SERVERS           = join(" ", var.servers)
  }

  artifactory = {
    username = var.artifactory_username
    token    = var.artifactory_token
    url      = var.artifact_url
    sha256   = var.artifact_sha
  }

  tls = {
    ca_file   = var.ca_file
    cert_file = var.cert_file
    key_file  = var.key_file
  }
}

resource "random_pet" "upgrade" {
}

resource "enos_local_exec" "wait_for_leader" {
  environment = local.nomad_env

  scripts = [abspath("${path.module}/scripts/wait_for_stable_cluster.sh")]
}

// Forcing a snapshot from the leader drives the cluster to store the most recent
// state and exercise the snapshot restore at least once when upgrading.
// The resulting file wont be used. 
// The stale flag defaults to "false" but it is included to reinforce the fact
// that it has to be taken from the leader for future readers.
resource "enos_local_exec" "take_cluster_snapshot" {
  depends_on = [enos_local_exec.wait_for_leader]

  environment = local.nomad_env

  inline = [
    "nomad operator snapshot save -stale=false ${random_pet.upgrade.id}-0.snap",
  ]
}

////////////////////////////////////////////////////////////////////////////////
//                    Upgrading the first server (leader)
////////////////////////////////////////////////////////////////////////////////
module upgrade_first_server {
  depends_on = [enos_local_exec.take_cluster_snapshot]

  source = "../upgrade_instance"

  nomad_addr          = var.nomad_addr
  tls                 = local.tls
  nomad_token         = var.nomad_token
  platform            = var.platform
  instance_address    = var.servers[0]
  ssh_key_path        = var.ssh_key_path
  artifactory_release = local.artifactory
}

resource "enos_local_exec" "first_leader_verification" {
  depends_on = [module.upgrade_first_server]

  environment = local.nomad_env

  scripts = [abspath("${path.module}/scripts/wait_for_stable_cluster.sh")]
}

////////////////////////////////////////////////////////////////////////////////
//                    Upgrading the second server
////////////////////////////////////////////////////////////////////////////////
module upgrade_second_server {
  depends_on = [enos_local_exec.first_leader_verification]

  source = "../upgrade_instance"

  nomad_addr          = var.nomad_addr
  tls                 = local.tls
  nomad_token         = var.nomad_token
  platform            = var.platform
  instance_address    = var.servers[1]
  ssh_key_path        = var.ssh_key_path
  artifactory_release = local.artifactory
}

resource "enos_local_exec" "second_leader_verification" {
  depends_on = [module.upgrade_second_server]

  environment = local.nomad_env

  scripts = [abspath("${path.module}/scripts/wait_for_stable_cluster.sh")]
}

////////////////////////////////////////////////////////////////////////////////
//                    Upgrading the third server
////////////////////////////////////////////////////////////////////////////////
module upgrade_third_server {
  depends_on = [enos_local_exec.second_leader_verification]

  source = "../upgrade_instance"

  nomad_addr          = var.nomad_addr
  tls                 = local.tls
  nomad_token         = var.nomad_token
  platform            = var.platform
  instance_address    = var.servers[2]
  ssh_key_path        = var.ssh_key_path
  artifactory_release = local.artifactory
}

resource "enos_local_exec" "last_leader_verification" {
  depends_on = [module.upgrade_third_server]

  environment = local.nomad_env

  scripts = [abspath("${path.module}/scripts/wait_for_stable_cluster.sh")]
}
