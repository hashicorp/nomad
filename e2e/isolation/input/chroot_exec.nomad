job "chroot_exec" {
  datacenters = ["dc1"]
  type        = "batch"
  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }
  group "print" {
    task "env" {
      driver = "exec"
      config {
        command = "/bin/bash"
        args = [
          "-c",
          "echo $NOMAD_ALLOC_DIR; echo $NOMAD_TASK_DIR; echo $NOMAD_SECRETS_DIR; echo $PATH"
        ]
      }
      resources {
        cpu    = 10
        memory = 16
      }
    }
  }
}
