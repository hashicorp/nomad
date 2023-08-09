# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "nomadservice" {
  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    update {
      min_healthy_time = "3s"
    }

    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    network {
      mode = "host"
      port "http" {}
    }

    task "pythonweb" {
      driver = "exec"
      config {
        command = "python"
        args    = ["-m", "http.server", "${NOMAD_PORT_http}", "--directory", "/tmp"]
      }
      resources {
        cpu    = 100
        memory = 32
      }
    }

    task "testcase" {
      driver = "exec"
      config {
        command = "cat"
        args = ["local/config.txt"]
      }
      template {
        destination = "config.txt"
        data = <<EOH
EOH
      }
    }
  }
}
