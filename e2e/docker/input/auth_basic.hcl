# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This job runs a docker task using a container stored in a private registry
# configured with basic authentication. The registry.hcl job should be running
# and healthy before running this job. The registry_address and registry_port
# HCL variables must be provided.

variable "registry_address" {
  type        = string
  description = "The HTTP address of the local registry"
  default     = "localhost"
}

variable "registry_port" {
  type        = number
  description = "The HTTP port of the local registry"
  default     = "7511"
}

variable "registry_username" {
  type        = string
  description = "The Basic Auth username of the local registry"
  default     = "auth_basic_user"
}

variable "registry_password" {
  type        = string
  description = "The Basic Auth password of the local registry"
  default     = "auth_basic_pass"
}

locals {
  registry_auth = base64encode("${var.registry_username}:${var.registry_password}")
}

job "auth_basic" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "basic" {
    reschedule {
      attempts  = 0
      unlimited = false
    }

    network {
      mode = "host"
    }

    task "echo" {
      driver = "docker"

      config {
        image          = "${var.registry_address}:${var.registry_port}/docker.io/library/bash_auth_basic:private"
        args           = ["echo", "The auth basic test is OK!"]
        auth_soft_fail = true

        auth {
          username = "${var.registry_username}"
          password = "${var.registry_password}"
        }
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
