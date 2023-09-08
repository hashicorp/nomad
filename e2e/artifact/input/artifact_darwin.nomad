# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# note: this job is not run regularly in e2e,
# where we have no macOS runner.

job "darwin" {
  datacenters = ["dc1"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "darwin"
  }

  group "rawexec" {
    task "rawexec" {
      artifact {
        source = "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod"
      }
      driver = "raw_exec"
      config {
        command = "cat"
        args    = ["local/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    task "rawexec_file_custom" {
      artifact {
        source      = "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod"
        destination = "local/my/path"
      }
      driver = "raw_exec"
      config {
        command = "cat"
        args    = ["local/my/path/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    task "rawexec_zip_default" {
      artifact {
        source = "https://github.com/hashicorp/go-set/archive/refs/heads/main.zip"
      }
      driver = "raw_exec"
      config {
        command = "cat"
        args    = ["local/go-set-main/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    task "rawexec_zip_custom" {
      artifact {
        source      = "https://github.com/hashicorp/go-set/archive/refs/heads/main.zip"
        destination = "local/my/zip"
      }
      driver = "raw_exec"
      config {
        command = "cat"
        args    = ["local/my/zip/go-set-main/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    task "rawexec_git_custom" {
      artifact {
        source      = "git::https://github.com/hashicorp/go-set"
        destination = "local/repository"
      }
      driver = "raw_exec"
      config {
        command = "cat"
        args    = ["local/repository/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }
  }
}
