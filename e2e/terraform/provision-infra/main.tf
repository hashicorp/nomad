# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

data "aws_caller_identity" "current" {
}

resource "random_pet" "e2e" {
}

resource "random_password" "windows_admin_password" {
  length           = 20
  special          = true
  override_special = "_%@"
}

locals {
  random_name = "${var.name}-${random_pet.e2e.id}"
  uploads_dir = "${path.module}/provision-nomad/uploads/${random_pet.e2e.id}"
  keys_dir    = "${path.module}/keys/${random_pet.e2e.id}"
}

# Generates keys to use for provisioning and access
module "keys" {
  depends_on = [random_pet.e2e]
  name       = local.random_name
  path       = "${local.keys_dir}"
  source     = "mitchellh/dynamic-keys/aws"
  version    = "v2.0.0"
}

data "aws_kms_alias" "e2e" {
  name = "alias/${var.aws_kms_alias}"
}
