# Copyright IBM Corp. 2015, 2025
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
}

resource "enos_local_exec" "wait_for_nomad_api" {
  environment = local.nomad_env
  scripts     = [abspath("${path.module}/scripts/wait_for_nomad_api.sh")]
}

resource "enos_local_exec" "run_tests" {
  depends_on = [enos_local_exec.wait_for_nomad_api]
  environment = merge(
    local.nomad_env, {
      JOBS          = join(",", var.jobs)
      SERVICE_JOBS  = join(",", var.service_jobs)
      SYSTEM_JOBS   = join(",", var.system_jobs)
      BATCH_JOBS    = join(",", var.batch_jobs)
      SYSBATCH_JOBS = join(",", var.sysbatch_jobs)
      CLIENT_COUNT  = var.client_count
  })

  scripts = [
    abspath("${path.module}/scripts/jobs.sh"),
    abspath("${path.module}/scripts/allocs.sh")
  ]
}
