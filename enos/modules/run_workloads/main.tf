# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

terraform {
  required_providers {
    enos = {
      source = "hashicorp-forge/enos"
    }
  }
}

resource "enos_local_exec" "wait_for_nomad_api" {   
    environment = {
      NOMAD_ADDR        = var.nomad_addr
      NOMAD_CACERT      = var.ca_file
      NOMAD_CLIENT_CERT = var.cert_file
      NOMAD_CLIENT_KEY  = var.key_file
      NOMAD_TOKEN       = var.nomad_token
    }

    inline = ["while ! nomad server members > /dev/null 2>&1; do echo 'waiting for nomad api...'; sleep 10; done"]
}

resource "local_file" "nomad_job_files" {
   for_each = var.workloads

  filename = "${path.module}/jobs/${each.key}.hcl"
  content  = templatefile(each.value.path, { alloc_count = each.value.alloc_count })
}

resource "enos_local_exec" "workloads" {
  for_each = var.workloads

  environment = {
      NOMAD_ADDR        = var.nomad_addr
      NOMAD_CACERT      = var.ca_file
      NOMAD_CLIENT_CERT = var.cert_file
      NOMAD_CLIENT_KEY  = var.key_file
      NOMAD_TOKEN       = var.nomad_token
   }

  inline = ["nomad job run ${path.module}/jobs/${each.key}.nomadhcl"]
}
