# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "setup-podman-auth" {
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

    restart {
      attempts = 0
      mode     = "fail"
    }

    task "create-auth-file" {
      driver = "raw_exec"
      user   = "root"

      config {
        command = "cp"
        args    = ["${NOMAD_TASK_DIR}/auth.json", "/etc/auth.json"]
      }

      template {
        destination = "local/auth.json"
        perms       = "644"
        data        = <<EOH
{}
EOH
      }

      resources {
        cpu    = 100
        memory = 32
      }
    }

    task "create-helper-file" {
      driver = "raw_exec"
      user   = "root"

      config {
        command = "cp"
        args    = ["${NOMAD_TASK_DIR}/test.sh", "/usr/local/bin/docker-credential-test.sh"]
      }

      template {
        destination = "local/test.sh"
        perms       = "755"
        data        = <<EOH
#!/usr/bin/env bash

set -euo pipefail

echo "{}"

exit 0
EOH
      }

      resources {
        cpu    = 100
        memory = 32
      }
    }
  }
}
