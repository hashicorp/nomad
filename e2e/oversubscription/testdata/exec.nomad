# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "oversubscription-exec" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    task "task" {
      driver = "exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "cat /proc/self/cgroup  | grep memory | cut -d: -f3 | tee ${NOMAD_ALLOC_DIR}/tmp/cgroup_name; sleep 1000"]
      }

      resources {
        cpu        = 500
        memory     = 20
        memory_max = 30
      }
    }

    task "cgroup-fetcher" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args = ["-c", <<EOF
until [ -s "${NOMAD_ALLOC_DIR}/tmp/cgroup_name" ]; do
  sleep 0.1
done

cat "/sys/fs/cgroup/memory/$(cat "${NOMAD_ALLOC_DIR}/tmp/cgroup_name" )/memory.limit_in_bytes" \
  | tee "${NOMAD_ALLOC_DIR}/tmp/memory.limit_in_bytes"

sleep 1000

EOF
        ]
      }

      resources {
        cpu    = 500
        memory = 20
      }
    }
  }
}
