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

resource "enos_local_exec" "wait_for_nomad_api" {
  environment = local.nomad_env

  scripts = [abspath("${path.module}/scripts/wait_for_nomad_api.sh")]
}

resource "enos_local_exec" "set_metadata" {
  depends_on = [enos_local_exec.wait_for_nomad_api]

  environment = merge(
    local.nomad_env,
    {
      CLIENT_IP = var.client
    }
  )

  scripts = [abspath("${path.module}/scripts/set_metadata.sh")]
}

resource "enos_local_exec" "get_alloc_ids" {

  environment = merge(
    local.nomad_env,
    {
      CLIENT_IP = var.client
    }
  )

  inline = [
    "nomad alloc status -json | jq -r --arg NODE_ID \"$(nomad node status -allocs -address https://$CLIENT_IP:4646 -self -json | jq -r '.ID')\" '[.[] | select(.ClientStatus == \"running\" and .NodeID == $NODE_ID) | .ID] | join(\" \")'"
  ]
}

module "upgrade_client" {
  depends_on = [
    enos_local_exec.set_metadata,
    enos_local_exec.get_alloc_ids,
  ]

  source = "../upgrade_instance"

  nomad_addr          = var.nomad_addr
  tls                 = local.tls
  nomad_token         = var.nomad_token
  platform            = var.platform
  instance_address    = var.client
  ssh_key_path        = var.ssh_key_path
  artifactory_release = local.artifactory
}

resource "enos_local_exec" "wait_for_nomad_api_post_update" {
  depends_on  = [module.upgrade_client]
  environment = local.nomad_env

  scripts = [abspath("${path.module}/scripts/wait_for_nomad_api.sh")]
}

resource "enos_local_exec" "verify_metadata" {
  depends_on = [enos_local_exec.wait_for_nomad_api_post_update]

  environment = merge(
    local.nomad_env,
    {
      CLIENT_IP = var.client
  })

  scripts = [abspath("${path.module}/scripts/verify_metadata.sh")]
}

resource "enos_local_exec" "verify_allocs" {
  depends_on = [enos_local_exec.wait_for_nomad_api_post_update]

  environment = merge(
    local.nomad_env,
    {
      CLIENT_IP = var.client
      ALLOCS    = enos_local_exec.get_alloc_ids.stdout
  })

  scripts = [abspath("${path.module}/scripts/verify_allocs.sh")]
}
