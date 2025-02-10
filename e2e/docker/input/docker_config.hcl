# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "registry_address" {
  type        = string
  description = "The HTTP address of the local registry"
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

job "configure-docker" {
  type = "sysbatch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "create-conf" {
    task "create-daemon-file" {
      driver = "pledge"
      user   = "${var.user}"

      config {
        command  = "cp"
        args     = ["${NOMAD_TASK_DIR}/daemon.json", "${var.docker_conf_dir}/daemon.json"]
        promises = "stdio rpath wpath cpath"
        unveil   = ["r:${NOMAD_TASK_DIR}/daemon.json", "rwc:${var.docker_conf_dir}"]
      }

      template {
        destination = "local/daemon.json"
        perms       = "644"
        data        = <<EOH
{
   "insecure-registries": [
      "${var.registry_address}"
   ]
}
EOH
      }
      resources {
        cpu    = 100
        memory = 32
      }
    }

    task "restart-docker" {
      driver = "raw_exec" # TODO: see if this could be done with pledge?

      config {
        command = "service"
        args    = ["docker", "restart"]
      }
      resources {
        cpu    = 100
        memory = 32
      }
    }
  }
}
