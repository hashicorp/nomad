# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "alloc_count" {
  type    = number
  default = 1
}

job "system-raw-exec" {
  type = "system"

  group "system-raw-exec" {

    network {
      port "server" {
        to = 0
      }
    }

    service {
      provider = "consul"
      name     = "system-raw-exec"

      check {
        name     = "system-raw-exec_probe"
        type     = "http"
        path     = "/index.html"
        interval = "10s"
        timeout  = "1s"
        port     = "server"
      }
    }

    task "system-raw-exec" {
      driver = "raw_exec"

      config {
        command = "python3"
        args    = ["-m", "http.server", "${NOMAD_PORT_server}", "--directory", "local"]
      }

      template {
        data        = <<EOH
<!DOCTYPE html>
<html lang="en">
<head>
  <meta ="UTF-8">
  <meta name="jobName" content="{{env "NOMAD_JOB_NAME"}}">
  <meta name="nodeID" content="{{env "node.unique.id"}}">
  <meta name="allocID" content="{{env "NOMAD_ALLOC_ID"}}">
</head>
<body>
<h1>All good and running for {{env "NOMAD_JOB_NAME"}} at {{env "node.unique.id"}}!</h1>
</body>
</html>
EOH
        destination = "local/index.html"
        perms       = "0644"
      }

      resources {
        cpu    = 50
        memory = 64
      }
    }
  }
}
