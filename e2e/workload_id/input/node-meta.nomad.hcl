# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "foo_key" {
  type = string
}

variable "empty_key" {
  type = string
}

variable "unset_key" {
  type = string
}

variable "foo_constraint" {
  type = string
}

variable "empty_constraint" {
  type = string
}

variable "unset_constraint" {
  type = string
}

job "node-meta" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  constraint {
    attribute = var.foo_constraint
    value     = "bar"
  }

  constraint {
    attribute = var.empty_constraint
    operator  = "is_set"
  }

  constraint {
    attribute = var.unset_constraint
    operator  = "is_not_set"
  }

  group "node-meta" {

    // sets keyUnset
    task "docker-nm" {
      driver = "docker"

      config {
        image = "curlimages/curl:7.87.0"
        args = [
          "--unix-socket", "${NOMAD_SECRETS_DIR}/api.sock",
          "-H", "Authorization: Bearer ${NOMAD_TOKEN}",
          "--data-binary", "{\"Meta\": {\"${var.unset_key}\": \"set\"}}",
          "--fail-with-body",
          "--verbose",
          "localhost/v1/client/metadata",
        ]
      }

      identity {
        env = true
      }

      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    // unsets keyEmpty
    task "exec-nm" {
      driver = "exec"

      config {
        command = "curl"
        args = [
          "-H", "Authorization: Bearer ${NOMAD_TOKEN}",
          "--unix-socket", "${NOMAD_SECRETS_DIR}/api.sock",
          "--data-binary", "{\"Meta\": {\"${var.empty_key}\": null}}",
          "--fail-with-body",
          "--verbose",
          "localhost/v1/client/metadata",
        ]
      }

      identity {
        env = true
      }

      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }
  }
}
