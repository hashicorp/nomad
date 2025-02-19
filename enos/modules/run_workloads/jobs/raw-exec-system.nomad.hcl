# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "alloc_count" {
  type    = number
  default = 1
}

job "system-raw-exec" {
  type = "system"

  group "system-raw-exec" {

    task "system-raw-exec" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "./local/runme.sh"]
      }

      template {
        data        = <<EOH
#!/bin/bash

while true; do
    sleep 30000  
done
EOH
        destination = "local/runme.sh"
        perms       = "755"
      }

      resources {
        cpu    = 50
        memory = 64
      }      
    }
  }
}
