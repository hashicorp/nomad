# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "volumes" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    volume "data" {
      type   = "host"
      source = "shared_data"
    }

    task "docker_task" {

      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["/usr/local/bin/myapplication.sh"]

        mounts = [
          # this mount binds the task's own NOMAD_TASK_DIR directory as the
          # source, letting us map it to a more convenient location; this is a
          # frequently-used way to get templates into an arbitrary location in
          # the task for Docker
          {
            type     = "bind"
            source   = "local"
            target   = "/usr/local/bin"
            readonly = true
          }
        ]
      }

      # this is the host volume mount, which we'll write into in our task to
      # ensure we have persistent data
      volume_mount {
        volume      = "data"
        destination = "/tmp/foo"
      }

      template {
        data = <<EOT
#!/bin/sh
echo ${NOMAD_ALLOC_ID} > /tmp/foo/${NOMAD_ALLOC_ID}
sleep 3600
EOT

        # this path is relative to the allocation's task directory:
        # /var/nomad/alloc/:alloc_id/:task_name
        # but Docker tasks can't see this folder except for the bind-mounted
        # directories inside it (./local ./secrets ./tmp)
        # so the only reason this works to write our script to execute from
        # /usr/local/bin is because of the 'mounts' section above.
        destination = "local/myapplication.sh"
      }

      resources {
        cpu    = 256
        memory = 128
      }
    }

    task "exec_task" {

      driver = "exec"

      config {
        command = "/bin/sh"
        args    = ["/usr/local/bin/myapplication.sh"]
      }

      # host volumes for exec tasks are more limited, so we're only going to read
      # data that the other task places there
      #
      # - we can't write unless the nobody user has permissions to write there
      # - we can't template into this location because the host_volume mounts
      #   over the template (see https://github.com/hashicorp/nomad/issues/7796)
      volume_mount {
        volume      = "data"
        destination = "/tmp/foo"
        read_only   = true
      }

      template {
        data = <<EOT
#!/bin/sh
sleep 3600
EOT

        # this path is relative to the allocation's task directory:
        # /var/nomad/alloc/:alloc_id/:task_name
        # which is the same as the root directory for exec tasks.
        # we just need to make sure this doesn't collide with the
        # chroot: https://www.nomadproject.io/docs/drivers/exec#chroot
        destination = "usr/local/bin/myapplication.sh"
      }

      resources {
        cpu    = 256
        memory = 128
      }
    }
  }
}
