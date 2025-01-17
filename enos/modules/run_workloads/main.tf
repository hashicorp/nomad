# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

terraform {
  required_providers {
    enos = {
      source = "hashicorp-forge/enos"
    }
  }
}

locals {
  clean_token = trimspace(var.nomad_token) #Somewhere in the process, a newline is added to teh token.
}

resource "enos_local_exec" "wait_for_nomad_api" {   
    environment = {
      NOMAD_ADDR        = var.nomad_addr
      NOMAD_CACERT      = var.ca_file
      NOMAD_CLIENT_CERT = var.cert_file
      NOMAD_CLIENT_KEY  = var.key_file
      NOMAD_TOKEN       = local.clean_token
    }

    scripts = [abspath("${path.module}/scripts/wait_for_nomad_api.sh")]
}

resource "local_file" "nomad_job_files" {
   for_each = var.workloads

  filename = "${path.module}/jobs/${each.key}.nomad.hcl"
  content  = templatefile("${path.module}/${each.value.template}", { alloc_count = each.value.alloc_count })
}

resource "enos_local_exec" "workloads" {
  for_each = var.workloads

  environment = {
      NOMAD_ADDR        = var.nomad_addr
      NOMAD_CACERT      = var.ca_file
      NOMAD_CLIENT_CERT = var.cert_file
      NOMAD_CLIENT_KEY  = var.key_file
      NOMAD_TOKEN       = local.clean_token
   }

  inline = ["nomad job run ${path.module}/jobs/${each.key}.nomad.hcl"]
}
