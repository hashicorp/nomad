# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

terraform {
  required_providers {
    enos = {
      source = "hashicorp-forge/enos"
    }
  }
}

locals {
  service_jobs  = ["service-docker", "service-raw-exec", "writes-vars", "countdash"]
  system_jobs   = ["system-docker", "system-raw-exec"]
  batch_jobs    = ["batch-docker", "batch-raw-exec"]
  sysbatch_jobs = [] # TODO
}

locals {
  nomad_env = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
  }

  consul_env = {
    CONSUL_HTTP_TOKEN = var.consul_token
    CONSUL_CACERT     = var.ca_file
    CONSUL_HTTP_ADDR  = var.consul_addr
  }

  vault_env = {
    VAULT_TOKEN = var.vault_token
    VAULT_PATH  = var.vault_mount_path
    VAULT_ADDR  = var.vault_addr
  }

}

resource "enos_local_exec" "wait_for_nomad_api" {
  environment = local.nomad_env
  scripts     = [abspath("${path.module}/scripts/wait_for_nomad_api.sh")]
}

resource "local_file" "vault_workload" {
  filename = "${path.module}/jobs/vault-secrets.nomad.hcl"
  content = templatefile("${path.module}/templates/vault-secrets.nomad.hcl.tpl", {
    secret_path = "${var.vault_mount_path}/default/get-secret"
  })
}

resource "enos_local_exec" "workloads" {
  depends_on = [
    enos_local_exec.wait_for_nomad_api,
    local_file.vault_workload
  ]
  for_each = var.workloads

  environment = merge(
    local.nomad_env,
    local.vault_env,
    local.consul_env,
  )

  inline = [
    each.value.pre_script != null ? abspath("${path.module}/${each.value.pre_script}") : "echo ok",
    "nomad job run -var alloc_count=${each.value.alloc_count} ${path.module}/${each.value.job_spec}",
    each.value.post_script != null ? abspath("${path.module}/${each.value.post_script}") : "echo ok"
  ]
}
