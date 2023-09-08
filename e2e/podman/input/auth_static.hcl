# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This job runs a podman task using a container stored in a private registry
# configured with file config static authentication. The registry.hcl job should
# be running and healthy before running this job.

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

job "auth_static" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "static" {
    reschedule {
      attempts  = 0
      unlimited = false
    }

    network {
      mode = "host"
    }

    task "echo" {
      driver = "podman"

      config {
        image = "${var.registry_address}:${var.registry_port}/docker.io/library/bash_auth_static:private"
        args  = ["echo", "The static auth test is OK!"]

        auth {
          # usename and password come from auth.json in plugin config
          tls_verify = false
        }
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}

# auth.json (must be pointed to by config=<path>/auth.json)
#
# {
#   "auths": {
#     "127.0.0.1:7511/docker.io/library/bash_auth_static": {
#       "auth": "YXV0aF9zdGF0aWNfdXNlcjphdXRoX3N0YXRpY19wYXNz"
#     }
#   }
# }

