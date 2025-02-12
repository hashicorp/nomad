# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "alloc_count" {
  type    = number
  default = 1
}

job "service-raw" {

  group "service-raw" {
    count = var.alloc_count
    task "raw" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "./local/runme.sh"]
      }

      template {
        data        = <<EOH
 #!/bin/bash

sigkill_handler() {
    echo "Received SIGKILL signal. Exiting..."
    exit 0
}

echo "Sleeping until SIGKILL signal is received..."
while true; do
    sleep 300  
done
EOH
        destination = "local/runme.sh"
        perms       = "755"
      }
    }
  }
}
