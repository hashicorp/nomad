# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This job makes requests to the "python-http" service.

job "http_curl" {
  type = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "client" {
    task "curl" {
      driver = "exec2"

      config {
        command = "bash"
        args    = ["local/script.sh"]
      }

      template {
        destination = "local/script.sh"
        change_mode = "noop"
        data        = <<EOF
#!/usr/bin/env bash

while true
do

{{ range nomadService "python-http" }}
(curl -s -S -L "{{ .Address }}:{{ .Port }}/hi.html") || true
{{ end }}

sleep 2

done
EOF
      }
    }
  }
}
