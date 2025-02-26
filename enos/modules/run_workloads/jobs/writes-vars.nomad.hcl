# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "alloc_count" {
  type    = number
  default = 1
}

# a job that continuously writes a counter value to a Nomad Variable. This
# exercises Workload Identity and the Task API.
job "writes-vars" {

  group "group" {

    count = var.alloc_count

    task "task" {
      driver = "docker"

      config {
        image   = "curlimages/curl:latest"
        command = "/bin/sh"
        args    = ["/local/script.sh"]
      }

      template {
        destination = "local/script.sh"
        data        = <<EOT
count=0
while :
do
  count=$(( count + 1 ))
  body=$(printf '{"Items": {"count": "%d" }}' $count)
  echo "sending ==> $body"
  curl --unix-socket "${NOMAD_SECRETS_DIR}/api.sock" \
       -H "Authorization: Bearer ${NOMAD_TOKEN}" \
       -verbose \
       --fail-with-body \
       -d "$body" \
       http://localhost/v1/var/nomad/jobs/writes-vars
  sleep 1
done

EOT

      }

      identity {
        env = true
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
