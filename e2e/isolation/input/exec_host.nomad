job "exec" {
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
          "-c", "local/pid.sh"
        ]
        pid_mode = "host"
        ipc_mode = "host"
      }

      template {
        data = <<EOF
#!/usr/bin/env bash
echo my pid is $BASHPID
EOF

        destination = "local/pid.sh"
        perms       = "777"
        change_mode = "noop"
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
