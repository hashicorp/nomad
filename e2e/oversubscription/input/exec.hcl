# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "oversubscription-exec" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    operator  = "="
    value     = "linux"
  }

  constraint {
    attribute = "${attr.os.cgroups.version}"
    operator  = "="
    value     = "2"
  }

  group "group" {
    task "sleep" {
      driver = "exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "sleep infinity"]
      }

      resources {
        cpu        = 500
        memory     = 20
        memory_max = 30
      }
    }

    task "cat" {
      driver = "pledge"

      lifecycle {
        hook = "poststart"
      }

      config {
        command = "/bin/cat"
        args    = ["/sys/fs/cgroup/nomad.slice/share.slice/${NOMAD_ALLOC_ID}.sleep.scope/memory.max"]
        unveil  = ["r:/sys/fs/cgroup/"]
      }

      resources {
        cpu    = 100
        memory = 20
      }
    }
  }
}
