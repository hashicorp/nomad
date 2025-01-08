# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

provider "nomad" {
  address   = "${var.nomad_addr}"
  ca_file   = "${var.ca_file}"
  cert_file = "${var.cert_file}"
  key_file  = "${var.key_file}"
  secret_id = "${var.nomad_token}"
}

resource "null_resource" "wait_for_nomad_api" {
  provisioner "local-exec" {
    command = "while ! nomad server members > /dev/null 2>&1; do echo 'waiting for nomad api...'; sleep 10; done"
    environment = {
      NOMAD_ADDR        = var.nomad_addr
      NOMAD_CACERT      = var.ca_file
      NOMAD_CLIENT_CERT = var.cert_file
      NOMAD_CLIENT_KEY  = var.key_file
      NOMAD_TOKEN       = var.nomad_token
    }
  }
}

resource "nomad_job" "workloads" {
  for_each = var.workloads
  jobspec  = templatefile(each.value.path, { alloc_count = each.value.alloc_count })
}