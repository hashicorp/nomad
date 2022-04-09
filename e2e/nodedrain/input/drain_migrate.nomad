job "drain_migrate" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    ephemeral_disk {
      migrate = true
      size    = "101"
    }

    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["local/test.sh"]
      }

      template {
        data = <<EOT
#!/bin/sh
if [ ! -f /alloc/data/{{ env "NOMAD_JOB_NAME" }} ]; then
  echo writing {{ env "NOMAD_ALLOC_ID" }} to /alloc/data/{{ env "NOMAD_JOB_NAME" }}
  echo {{ env "NOMAD_ALLOC_ID" }} > /alloc/data/{{ env "NOMAD_JOB_NAME" }}
else
   echo /alloc/data/{{ env "NOMAD_JOB_NAME" }} already exists
fi
sleep 3600
EOT

        destination = "local/test.sh"
      }

      resources {
        cpu    = 256
        memory = 128
      }
    }
  }
}
