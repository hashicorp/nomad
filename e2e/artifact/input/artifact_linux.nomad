# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "linux" {
  datacenters = ["dc1"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "rawexec" {

    task "rawexec_file_default" {
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

    task "rawexec_file_alloc_dots" {
      artifact {
        source      = "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod"
        destination = "../alloc/go.mod"
        mode        = "file"
      }
      driver = "raw_exec"
      config {
        command = "cat"
        args    = ["../alloc/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    task "rawexec_file_alloc_env" {
      artifact {
        source      = "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod"
        destination = "${NOMAD_ALLOC_DIR}/go.mod"
        mode        = "file"
      }
      driver = "raw_exec"
      config {
        command = "cat"
        args    = ["${NOMAD_ALLOC_DIR}/go.mod"]
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

  group "exec" {
    task "exec_file_default" {
      artifact {
        source = "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod"
      }
      driver = "exec"
      config {
        command = "cat"
        args    = ["local/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 256
      }
    }

    task "exec_file_custom" {
      artifact {
        source      = "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod"
        destination = "local/my/path"
      }
      driver = "exec"
      config {
        command = "cat"
        args    = ["local/my/path/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 256
      }
    }

    task "exec_file_alloc" {
      artifact {
        source      = "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod"
        destination = "${NOMAD_ALLOC_DIR}/go.mod"
        mode        = "file"
      }
      driver = "exec"
      config {
        command = "cat"
        args    = ["${NOMAD_ALLOC_DIR}/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    task "exec_zip_default" {
      artifact {
        source = "https://github.com/hashicorp/go-set/archive/refs/heads/main.zip"
      }
      driver = "exec"
      config {
        command = "cat"
        args    = ["local/go-set-main/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 256
      }
    }

    task "exec_zip_custom" {
      artifact {
        source      = "https://github.com/hashicorp/go-set/archive/refs/heads/main.zip"
        destination = "local/my/zip"
      }
      driver = "exec"
      config {
        command = "cat"
        args    = ["local/my/zip/go-set-main/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 256
      }
    }

    task "exec_git_custom" {
      artifact {
        source      = "git::https://github.com/hashicorp/go-set"
        destination = "local/repository"
      }
      driver = "exec"
      config {
        command = "cat"
        args    = ["local/repository/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 256
      }
    }
  }

  group "docker" {
    task "docker_file_default" {
      artifact {
        source = "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod"
      }
      driver = "docker"
      config {
        image = "bash:5"
        args  = ["cat", "local/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    task "docker_file_custom" {
      artifact {
        source      = "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod"
        destination = "local/my/path"
      }
      driver = "docker"
      config {
        image = "bash:5"
        args  = ["cat", "local/my/path/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    task "docker_file_alloc" {
      artifact {
        source      = "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod"
        destination = "${NOMAD_ALLOC_DIR}/go.mod"
        mode        = "file"
      }
      driver = "docker"
      config {
        image = "bash:5"
        args  = ["cat", "${NOMAD_ALLOC_DIR}/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    task "docker_zip_default" {
      artifact {
        source = "https://github.com/hashicorp/go-set/archive/refs/heads/main.zip"
      }
      driver = "docker"
      config {
        image = "bash:5"
        args  = ["cat", "local/go-set-main/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    task "docker_zip_custom" {
      artifact {
        source      = "https://github.com/hashicorp/go-set/archive/refs/heads/main.zip"
        destination = "local/my/zip"
      }
      driver = "docker"
      config {
        image = "bash:5"
        args  = ["cat", "local/my/zip/go-set-main/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    task "docker_git_custom" {
      artifact {
        source      = "git::https://github.com/hashicorp/go-set"
        destination = "local/repository"
      }
      driver = "docker"
      config {
        image = "bash:5"
        args  = ["cat", "local/repository/go.mod"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }
  }
}
