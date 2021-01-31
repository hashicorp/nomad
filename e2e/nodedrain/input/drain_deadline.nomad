job "drain_deadline" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    task "task" {
      driver = "docker"

      kill_timeout = "2m"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["local/script.sh"]
      }

      template {
        data = <<EOF
#!/bin/sh
trap 'sleep 60' 2
sleep 600
EOF

        destination = "local/script.sh"
        change_mode = "noop"
      }

      resources {
        cpu    = 256
        memory = 128
      }
    }
  }
}
