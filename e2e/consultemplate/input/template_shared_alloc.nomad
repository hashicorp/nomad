job "template-shared-alloc" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "template-paths" {

    task "raw_exec" {
      driver = "raw_exec"
      config {
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      artifact {
        source      = "https://google.com"
        destination = "../alloc/google1.html"
      }

      template {
        destination = "${NOMAD_ALLOC_DIR}/${NOMAD_TASK_NAME}.env"
        data        = <<EOH
{{env "NOMAD_ALLOC_DIR"}}
EOH
      }

      resources {
        cpu    = 128
        memory = 64
      }

    }

    task "docker" {
      driver = "docker"
      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      artifact {
        source      = "https://google.com"
        destination = "../alloc/google2.html"
      }

      template {
        destination = "${NOMAD_ALLOC_DIR}/${NOMAD_TASK_NAME}.env"
        data        = <<EOH
{{env "NOMAD_ALLOC_DIR"}}
EOH
      }

      resources {
        cpu    = 128
        memory = 64
      }
    }

    task "exec" {
      driver = "exec"
      config {
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      artifact {
        source      = "https://google.com"
        destination = "${NOMAD_ALLOC_DIR}/google3.html"
      }

      template {
        destination = "${NOMAD_ALLOC_DIR}/${NOMAD_TASK_NAME}.env"
        data        = <<EOH
{{env "NOMAD_ALLOC_DIR"}}
EOH
      }

      resources {
        cpu    = 128
        memory = 64
      }
    }

  }
}
