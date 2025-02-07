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

#////////////////////////////////////////////////////////////////////////////////
#//                    Upgrading the first client
#////////////////////////////////////////////////////////////////////////////////

resource "enos_local_exec" "set_metadata_on_first_client" {
  depends_on = [enos_local_exec.wait_for_nomad_api]

  environment = merge(
    local.nomad_env,
    {
      CLIENT_IP = var.clients[0]
    }
  )

  scripts = [abspath("${path.module}/scripts/set_metadata.sh")]
}

module upgrade_first_client {
  depends_on = [enos_local_exec.set_metadata_on_first_client]

  source = "../upgrade_instance"

  nomad_addr          = var.nomad_addr
  tls                 = local.tls
  nomad_token         = var.nomad_token
  platform            = var.platform
  instance_address    = var.clients[0]
  ssh_key_path        = var.ssh_key_path
  artifactory_release = local.artifactory
}

resource "enos_local_exec" "verify_metadata_from_first_client" {
  depends_on = [module.upgrade_first_client]

  environment = merge(
    local.nomad_env,
    {
      CLIENT_IP = var.clients[0]
  })

  scripts = [abspath("${path.module}/scripts/verify_metadata.sh")]
}

#////////////////////////////////////////////////////////////////////////////////
#//                    Upgrading the second client
#////////////////////////////////////////////////////////////////////////////////

resource "enos_local_exec" "set_metadata_on_second_client" {
  depends_on = [enos_local_exec.verify_metadata_from_first_client]

  environment = merge(
    local.nomad_env,
    {
      CLIENT_IP = var.clients[1]
    }
  )

  scripts = [abspath("${path.module}/scripts/set_metadata.sh")]
}

module upgrade_second_client {
  depends_on = [enos_local_exec.set_metadata_on_second_client]

  source = "../upgrade_instance"

  nomad_addr          = var.nomad_addr
  tls                 = local.tls
  nomad_token         = var.nomad_token
  platform            = var.platform
  instance_address    = var.clients[1]
  ssh_key_path        = var.ssh_key_path
  artifactory_release = local.artifactory
}

resource "enos_local_exec" "verify_metadata_from_second_client" {
  depends_on = [module.upgrade_second_client]

  environment = merge(
    local.nomad_env,
    {
      CLIENT_IP = var.clients[1]
  })

  scripts = [abspath("${path.module}/scripts/verify_metadata.sh")]
}

#////////////////////////////////////////////////////////////////////////////////
#//                    Upgrading the third client
#////////////////////////////////////////////////////////////////////////////////

resource "enos_local_exec" "set_metadata_on_third_client" {
  depends_on = [enos_local_exec.verify_metadata_from_second_client]

  environment = merge(
    local.nomad_env,
    {
      CLIENT_IP = var.clients[2]
    }
  )

  scripts = [abspath("${path.module}/scripts/set_metadata.sh")]
}

module upgrade_third_client {
  depends_on = [enos_local_exec.set_metadata_on_third_client]

  source = "../upgrade_instance"

  nomad_addr          = var.nomad_addr
  tls                 = local.tls
  nomad_token         = var.nomad_token
  platform            = var.platform
  instance_address    = var.clients[2]
  ssh_key_path        = var.ssh_key_path
  artifactory_release = local.artifactory
}

resource "enos_local_exec" "verify_metadata_from_third_client" {
  depends_on = [module.upgrade_third_client]

  environment = merge(
    local.nomad_env,
    {
      CLIENT_IP = var.clients[2]
  })

  scripts = [abspath("${path.module}/scripts/verify_metadata.sh")]
}

#////////////////////////////////////////////////////////////////////////////////
#//                    Upgrading the forth client
#////////////////////////////////////////////////////////////////////////////////

resource "enos_local_exec" "set_metadata_on_forth_client" {
  depends_on = [enos_local_exec.verify_metadata_from_third_client]

  environment = merge(
    local.nomad_env,
    {
      CLIENT_IP = var.clients[3]
    }
  )

  scripts = [abspath("${path.module}/scripts/set_metadata.sh")]
}

module upgrade_forth_client {
  depends_on = [enos_local_exec.set_metadata_on_forth_client]

  source = "../upgrade_instance"

  nomad_addr          = var.nomad_addr
  tls                 = local.tls
  nomad_token         = var.nomad_token
  platform            = var.platform
  instance_address    = var.clients[3]
  ssh_key_path        = var.ssh_key_path
  artifactory_release = local.artifactory
}

resource "enos_local_exec" "verify_metadata_from_forth_client" {
  depends_on = [module.upgrade_forth_client]

  environment = merge(
    local.nomad_env,
    {
      CLIENT_IP = var.clients[3]
  })

  scripts = [abspath("${path.module}/scripts/verify_metadata.sh")]
}
