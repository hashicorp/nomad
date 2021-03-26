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
        args    = ["-c", "cat /sys/fs/cgroup/memory/memory.limit_in_bytes; sleep 1000"]
      }

      resources {
        cpu        = 500
        memory     = 20
        memory_max = 30
      }
    }
  }
}
