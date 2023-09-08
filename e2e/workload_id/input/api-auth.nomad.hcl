# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

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
        image = "curlimages/curl:7.87.0"
        args = [
          "--unix-socket", "${NOMAD_SECRETS_DIR}/api.sock",
          "-v",
          "localhost/v1/agent/health",
        ]
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
        image = "curlimages/curl:7.87.0"
        args = [
          "--unix-socket", "${NOMAD_SECRETS_DIR}/api.sock",
          "-H", "X-Nomad-Token: 37297754-3b87-41da-9ac7-d98fd934deed",
          "-v",
          "localhost/v1/agent/health",
        ]
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
        image = "curlimages/curl:7.87.0"
        args = [
          "--unix-socket", "${NOMAD_SECRETS_DIR}/api.sock",
          "-H", "Authorization: Bearer ${NOMAD_TOKEN}",
          "-v",
          "localhost/v1/agent/health",
        ]
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
        command = "curl"
        args = [
          "-H", "Authorization: Bearer ${NOMAD_TOKEN}",
          "--unix-socket", "${NOMAD_SECRETS_DIR}/api.sock",
          "-v",
          "localhost/v1/agent/health",
        ]
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
