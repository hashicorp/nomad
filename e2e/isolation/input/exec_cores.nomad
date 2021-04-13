job "exec-cores" {
  datacenters = ["dc1"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "exec" {
    task "exec" {
      driver = "exec"

      config {
        command = "bash"
        args = [
          "-c", "local/cgroups.sh"
        ]
      }

      template {
        data = <<EOF
#!/usr/bin/env bash
grep cpuset /proc/self/cgroup
EOF

        destination = "local/cgroups.sh"
        perms       = "777"
        change_mode = "noop"
      }

      resources {
        core = 1
        memory = 64
      }
    }
  }
}
