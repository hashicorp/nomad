# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

terraform "default" {
  required_version = ">= 1.2.0"

  required_providers {
    aws = {
      source = "hashicorp/aws"
    }

    enos = {
      source  = "registry.terraform.io/hashicorp-forge/enos"
      version = ">= 0.4.0"
    }
  }
}
