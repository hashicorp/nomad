# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# lifecycle hook test job for batch jobs. touches, removes, and tests
# for the existence of files to assert the order of running tasks.
# all tasks should exit 0 and the alloc dir should contain the following
# files: ./init-ran, ./main-ran, ./poststart-run

job "batch-lifecycle" {

  datacenters = ["dc1"]

  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "test" {

    task "init" {

      lifecycle {
        hook = "prestart"
      }

      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["local/prestart.sh"]
      }

      template {
        data = <<EOT
#!/bin/sh
sleep 1
touch ${NOMAD_ALLOC_DIR}/init-ran
touch ${NOMAD_ALLOC_DIR}/init-running
sleep 5
if [ -f ${NOMAD_ALLOC_DIR}/main ]; then exit 7; fi
if [ -f ${NOMAD_ALLOC_DIR}/poststart-running ]; then exit 8; fi
rm ${NOMAD_ALLOC_DIR}/init-running
EOT

        destination = "local/prestart.sh"

      }

      resources {
        cpu    = 64
        memory = 64
      }
    }

    task "main" {

      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["local/main.sh"]
      }

      template {
        data = <<EOT
#!/bin/sh
sleep 1
touch ${NOMAD_ALLOC_DIR}/main-running
touch ${NOMAD_ALLOC_DIR}/main-started
# NEED TO HANG AROUND TO GIVE POSTSTART TIME TO GET STARTED
sleep 10
if [ ! -f ${NOMAD_ALLOC_DIR}/init-ran ]; then exit 9; fi
if [ -f ${NOMAD_ALLOC_DIR}/init-running ]; then exit 10; fi

if [ ! -f ${NOMAD_ALLOC_DIR}/poststart-started ]; then exit 11; fi

touch ${NOMAD_ALLOC_DIR}/main-ran
rm ${NOMAD_ALLOC_DIR}/main-running
EOT

        destination = "local/main.sh"
      }

      resources {
        cpu    = 64
        memory = 64
      }
    }


    task "poststart" {

      lifecycle {
        hook = "poststart"
      }

      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["local/poststart.sh"]
      }

      template {
        data = <<EOT
#!/bin/sh
sleep 1
touch ${NOMAD_ALLOC_DIR}/poststart-ran
touch ${NOMAD_ALLOC_DIR}/poststart-running
touch ${NOMAD_ALLOC_DIR}/poststart-started
sleep 10

if [ ! -f ${NOMAD_ALLOC_DIR}/init-ran ]; then exit 12; fi
if [ ! -f ${NOMAD_ALLOC_DIR}/main-started ]; then exit 15; fi
if [ -f ${NOMAD_ALLOC_DIR}/init-running ]; then exit 14; fi
rm ${NOMAD_ALLOC_DIR}/poststart-running
EOT

        destination = "local/poststart.sh"
      }

      resources {
        cpu    = 64
        memory = 64
      }
    }

    task "poststop" {

      lifecycle {
        hook = "poststop"
      }

      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["local/poststop.sh"]
      }

      template {
        data = <<EOT
#!/bin/sh
sleep 1
touch ${NOMAD_ALLOC_DIR}/poststop-ran
touch ${NOMAD_ALLOC_DIR}/poststop-running
touch ${NOMAD_ALLOC_DIR}/poststop-started
sleep 5

if [ ! -f ${NOMAD_ALLOC_DIR}/init-ran ]; then exit 12; fi
if [ ! -f ${NOMAD_ALLOC_DIR}/main-started ]; then exit 15; fi
if [ -f ${NOMAD_ALLOC_DIR}/init-running ]; then exit 14; fi
if [ -f ${NOMAD_ALLOC_DIR}/main-running ]; then exit 17; fi
rm ${NOMAD_ALLOC_DIR}/poststop-running
EOT

        destination = "local/poststop.sh"
      }

      resources {
        cpu    = 64
        memory = 64
      }
    }
  }
}
