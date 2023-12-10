# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "alloc-logs" {
  datacenters = ["dc1"]
  type        = "service"

  #constraint {
  #  attribute = "${attr.kernel.name}"
  #  value     = "linux"
  #}

  group "alloc-logs" {

    task "test" {
      driver = "raw_exec"

      template {
        data = <<EOH
while true
do
  echo stdout >&1
  sleep 1
  echo stderr >&2
  sleep 1
done
EOH

        destination = "local/echo.sh"
      }

      config {
        command = "bash"
        args    = ["local/echo.sh"]
      }
    }
  }
}
