# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Serve the contents of our CSI volume with a little web server.
job "web" {
  group "web" {
    count = 2

    # request the volume; node plugin will provide it
    volume "csi-nfs" {
      type            = "csi"
      source          = "csi-nfs"
      attachment_mode = "file-system"
      access_mode     = "multi-node-multi-writer"
    }

    network {
      mode = "bridge"
      port "http" {
        to = 80
      }
    }
    service {
      provider = "nomad"
      name     = "web"
      port     = "http"
      check {
        type     = "http"
        path     = "/"
        interval = "2s"
        timeout  = "1s"
      }
    }

    task "web" {
      driver = "docker"

      # mount the volume!
      volume_mount {
        volume      = "csi-nfs"
        destination = "${NOMAD_ALLOC_DIR}/web-nfs"
      }

      # this host user:group maps back to volume parameters.
      user = "1000:1000"

      config {
        image   = "python:slim"
        command = "/bin/bash"
        args    = ["-x", "local/entrypoint.sh"]
        ports   = ["http"]
      }
      # this entrypoint writes `date` to index.html only on the first run,
      # to demonstrate that state is persisted in NFS across restarts, etc.
      # afterwards, this can also be seen on the host machine in
      #   /srv/host-nfs/csi-nfs/index.html
      # or in the other locations node plugin mounts on the host for this task.
      #   $ grep csi-nfs /proc/mounts
      template {
        destination = "local/entrypoint.sh"
        data        = <<EOF
#!/bin/bash
dir="${NOMAD_ALLOC_DIR}/web-nfs"
test -f $dir/index.html || echo hello from $(date) > $dir/index.html
python -m http.server ${NOMAD_PORT_http} --directory=$dir
EOF
      }
    }
  }
}
