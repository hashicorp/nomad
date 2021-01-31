provider "aws" {
  region = var.region

  assume_role {
    role_arn     = var.aws_assume_role_arn
    session_name = var.aws_assume_role_session_name
    external_id  = var.aws_assume_role_external_id
  }
}

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
}

# Generates keys to use for provisioning and access
module "keys" {
  name    = local.random_name
  path    = "${path.root}/keys"
  source  = "mitchellh/dynamic-keys/aws"
  version = "v2.0.0"
}
