# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "api-win" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "windows"
  }

  constraint {
    attribute = "${attr.cpu.arch}"
    value     = "amd64"
  }

  group "api-win" {

    task "win" {
      driver = "raw_exec"
      config {
        command = "powershell"
        args    = ["curl.exe -H \"Authorization: Bearer $env:NOMAD_TOKEN\" --unix-socket $env:NOMAD_SECRETS_DIR/api.sock -v localhost:4646/v1/agent/health"]
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
