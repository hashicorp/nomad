# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

locals {
  # these include a sleep, so docker logs can consistently be retrieved
  no_token_401 = <<-SCRIPT
    curl -v \
      --unix-socket ${NOMAD_SECRETS_DIR}/api.sock \
      localhost/v1/agent/health
    sleep 1
  SCRIPT

  bad_token_403 = <<-SCRIPT
    curl -v \
      --unix-socket ${NOMAD_SECRETS_DIR}/api.sock \
      --header "X-Nomad-Token: 37297754-3b87-41da-9ac7-d98fd934deed" \
      localhost/v1/agent/health
    sleep 1
  SCRIPT

  good_token = <<-SCRIPT
    curl -v \
      --unix-socket ${NOMAD_SECRETS_DIR}/api.sock \
      --header "X-Nomad-Token: ${NOMAD_TOKEN}" \
      localhost/v1/agent/health
    sleep 1
  SCRIPT
}

job "api-auth" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "api-auth" {

    # none task should get a 401 response
    task "none" {
      driver = "docker"
      config {
        image   = "curlimages/curl:7.87.0"
        command = "sh"
        args    = ["-c", "${local.no_token_401}"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    # bad task should get a 403 response
    task "bad" {
      driver = "docker"
      config {
        image   = "curlimages/curl:7.87.0"
        command = "sh"
        args    = ["-c", "${local.bad_token_403}"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    # docker-wid task should succeed due to using workload identity
    task "docker-wid" {
      driver = "docker"

      config {
        image   = "curlimages/curl:7.87.0"
        command = "sh"
        args    = ["-c", "${local.good_token}"]
      }

      identity {
        env = true
      }

      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    # exec-wid task should succeed due to using workload identity
    task "exec-wid" {
      driver = "exec"

      config {
        command = "sh"
        args    = ["-c", "${local.good_token}"]
      }

      identity {
        env = true
      }

      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }
  }
}
