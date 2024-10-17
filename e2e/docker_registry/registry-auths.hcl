# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This job runs after the private registry is up and running, when we know
# address and port provided by the bridge network. It is a sysbatch job
# that writes these files on every linux client.
#   - /usr/local/bin/docker-credential-test.sh
#   - /etc/docker-registry-auth.json

variable "registry_address" {
  type        = string
  description = "The HTTP address of the local registry"
}

variable "auth_dir" {
  type        = string
  description = "The destination directory of the auth.json file."
  default     = "/tmp"
}

variable "helper_dir" {
  type        = string
  description = "The directory in which test.sh will be written."
  default     = "/tmp"
}

variable "docker_conf_dir" {
  type        = string
  description = "The directory in which daemon.json will be written."
  default     = "/tmp"
}

variable "user" {
  type        = string
  description = "The user to create files as. Should be root in e2e."
  # no default because dealing with root files is annoying locally
  # try -var=user=$USER for local development
}

job "registry-auths" {
  type = "sysbatch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "create-files" {
    reschedule {
      attempts  = 0
      unlimited = false
    }

    # write out the test.sh file into var.helper_dir
    task "create-helper-file" {
      driver = "pledge"
      user   = "${var.user}"

      config {
        command  = "cp"
        args     = ["${NOMAD_TASK_DIR}/test.sh", "${var.helper_dir}/docker-credential-test.sh"]
        promises = "stdio rpath wpath cpath"
        unveil   = ["r:${NOMAD_TASK_DIR}/test.sh", "rwc:${var.helper_dir}"]
      }

      template {
        destination = "local/test.sh"
        perms       = "755"
        data        = <<EOH
#!/usr/bin/env bash

set -euo pipefail

value=$(cat /dev/stdin)

username="auth_helper_user"
password="auth_helper_pass"

case "${value}" in
  ${var.registry_address}*)
    echo "{\"Username\": \"$username\", \"Secret\": \"$password\"}"
    exit 0
    ;;
  *)
    echo "must use local registry"
    exit 3
    ;;
esac
EOH
      }
      resources {
        cpu    = 100
        memory = 32
      }
    }

    # write out the auth.json file into var.auth_dir
    task "create-auth-file" {
      driver = "pledge"
      user   = "${var.user}"

      config {
        command  = "cp"
        args     = ["${NOMAD_TASK_DIR}/auth.json", "${var.auth_dir}/auth.json"]
        promises = "stdio rpath wpath cpath"
        unveil   = ["r:${NOMAD_TASK_DIR}/auth.json", "rwc:${var.auth_dir}"]
      }
      template {
        perms       = "644"
        destination = "local/auth.json"
        data        = <<EOH
{
  "auths": {
    "${var.registry_address}:/docker.io/library/bash_auth_static": {
      "auth": "YXV0aF9zdGF0aWNfdXNlcjphdXRoX3N0YXRpY19wYXNz"
    }
  }
}
EOH
      }
      resources {
        cpu    = 100
        memory = 32
      }
    }
  }
}
