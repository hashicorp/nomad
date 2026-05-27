# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: MPL-2.0

job "snapshot-agent" {
  namespace = "infra"
  type      = "batch"

  periodic {
    crons            = ["*/5 * * * *"] # set this to something reasonable
    prohibit_overlap = true
  }

  group "group" {

    task "agent" {

      driver = "docker"

      config {
        # only Nomad Enterprise provides the snapshot agent command
        image = "hashicorp/nomad:2.0.0-ent"

        # see https://developer.hashicorp.com/nomad/commands/operator/snapshot/agent
        # for configuration options
        args = [
          "operator", "snapshot", "agent",
          "-interval", "0",            # one-shot for periodic batch job
          "-local-path", "/snapshots", # demo-only, use s3 or something
        ]
      }

      env {
        NOMAD_ADDR                   = "${NOMAD_UNIX_ADDR}"
        NOMAD_SKIP_DOCKER_IMAGE_WARN = "1"
      }

      # give the job NOMAD_TOKEN for a WI with a workload-associated policy
      identity {
        env = true
      }

      # you'll need to verify these values work for your environment.
      # see also: https://github.com/hashicorp/nomad/issues/27902
      resources {
        cpu    = 256
        memory = 256
      }

    }
  }
}
