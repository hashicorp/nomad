/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default `job "parameterized-job" {
  // Specifies the datacenter where this job should be run
  // This can be omitted and it will default to ["*"]
  datacenters = ["*"]

  // Unlike service jobs, Batch jobs are intended to run until they exit successfully.
  type = "batch"

  // Run the job only on Linux or MacOS.
  constraint {
    attribute = "\${attr.kernel.name}"
    operator  = "set_contains_any"
    value     = "darwin,linux"
  }

  // Allow the job to be parameterized, and allow any meta key with
  // a name starting with "i" to be specified.
  parameterized {
    meta_optional = ["MY_META_KEY"]
  }

  group "group" {
    task "task" {
      driver = "docker"
      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "cat local/template.out", "local/payload.txt"]
      }

      dispatch_payload {
        file = "payload.txt"
      }

      template {
        data = <<EOH
MY_META_KEY: {{env "NOMAD_META_MY_META_KEY"}}
  EOH

        destination = "local/template.out"
      }
    }
  }
}`;
