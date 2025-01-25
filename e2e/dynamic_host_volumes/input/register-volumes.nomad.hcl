// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

job "register-volumes" {

  type = "batch"

  parameterized {
    meta_required = ["vol_name", "vol_size", "vol_path"]
  }

  group "group" {

    restart {
      attempts = 0
      mode     = "fail"
    }

    task "task" {

      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["${NOMAD_TASK_DIR}/register.sh", "${node.unique.id}"]
      }

      template {
        destination = "local/register.sh"
        data        = <<EOT
#!/usr/bin/env bash
set -ex

export NOMAD_ADDR="unix://${NOMAD_SECRETS_DIR}/api.sock"
NODE_ID=$1
mkdir -p "${NOMAD_META_vol_path}"

sed -e "s~NODE_ID~$NODE_ID~" \
    -e "s~VOL_NAME~${NOMAD_META_vol_name}~" \
    -e "s~VOL_SIZE~${NOMAD_META_vol_size}~" \
    -e "s~VOL_PATH~${NOMAD_META_vol_path}~" \
    local/volume.hcl | nomad volume register -

        EOT

      }

      template {
        destination = "local/volume.hcl"
        data        = <<EOT
name      = "VOL_NAME"
node_id   = "NODE_ID"
type      = "host"
host_path = "VOL_PATH"
capacity  = "VOL_SIZE"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}
        EOT

      }


      identity {
        env = true
      }

      resources {
        cpu    = 100
        memory = 100
      }

    }
  }
}
