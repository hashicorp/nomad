# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "alloc_count" {
  type    = number
  default = 1
}

# a job that continuously writes a counter value to a Nomad Variable and reads
# it back out. This exercises Workload Identity and the Task API.
job "writes-vars" {

  group "group" {

    count = var.alloc_count

    # need a service port so we can have a health check, but it's not used here
    network {
      port "web" {
        to = 8001
      }
    }

    service {
      provider = "consul"
      name     = "writes-vars-checker"
      port     = "web"
      task     = "task"

      check {
        type     = "script"
        interval = "10s"
        timeout  = "1s"
        command  = "/bin/sh"
        args     = ["/local/read-script.sh"]

        # this check will read from the Task API, so we need to ensure that we
        # can tolerate the listener going away during client upgrades
        check_restart {
          limit = 10
        }
      }
    }


    task "task" {
      driver = "docker"

      config {
        image   = "curlimages/curl:latest"
        command = "/bin/sh"
        args    = ["/local/write-script.sh"]
      }

      template {
        destination = "local/write-script.sh"
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

      template {
        destination = "local/read-script.sh"
        data        = <<EOT
curl --unix-socket "${NOMAD_SECRETS_DIR}/api.sock" \
     -H "Authorization: Bearer ${NOMAD_TOKEN}" \
     -verbose \
     http://localhost/v1/var/nomad/jobs/writes-vars

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
