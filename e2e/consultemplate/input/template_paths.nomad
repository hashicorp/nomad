job "template-paths" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "template-paths" {

    task "task" {

      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      artifact {
        source      = "https://google.com"
        destination = "local/foo/src"
      }

      template {
        source      = "${NOMAD_TASK_DIR}/foo/src"
        destination = "${NOMAD_SECRETS_DIR}/foo/dst"
      }

      resources {
        cpu    = 128
        memory = 64
      }
    }
  }
}
