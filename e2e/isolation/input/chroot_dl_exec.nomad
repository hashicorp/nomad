# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "chroot_dl_exec" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    network {
      mode = "host"
      port "http" {}
    }

    task "script-writer" {
      driver = "raw_exec"
      config {
        command = "/bin/bash"
        args = [
          "-c",
          "cd ${NOMAD_ALLOC_DIR} && chmod +x script.sh && tar -czf script.tar.gz script.sh"
        ]
      }

      resources {
        cpu    = 50
        memory = 50
      }

      template {
        data        = <<EOH
#!/bin/sh
echo this output is from a script
EOH
        destination = "${NOMAD_ALLOC_DIR}/script.sh"
      }
      lifecycle {
        hook    = "prestart"
        sidecar = false
      }
    }

    task "file-server" {
      driver = "raw_exec"
      config {
        command = "/usr/bin/python3"
        args    = ["-m", "http.server", "${NOMAD_PORT_http}", "--directory", "${NOMAD_ALLOC_DIR}"]
      }

      resources {
        cpu    = 50
        memory = 50
      }
      lifecycle {
        hook    = "prestart"
        sidecar = true
      }
    }

    task "run-script" {
      driver = "exec"
      config {
        command = "local/script.sh"
      }
      resources {
        cpu    = 50
        memory = 50
      }
      artifact {
        source = "http://localhost:${NOMAD_PORT_http}/script.tar.gz"
      }
    }
  }
}
