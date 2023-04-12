# demo: serve the contents of our CSI volume with a little web server
job "web" {
  type = "service"
  group "web" {
    # request the volume
    volume "my-web-nfs" {
      type            = "csi"
      source          = "my-nfs"
      attachment_mode = "file-system"
      access_mode     = "multi-node-multi-writer"
      read_only       = false
    }
    network {
      mode = "bridge"
      port "http" {
        static = 8080
        to     = 80
      }
    }
    service { # mainly just for the health check
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
        volume      = "my-web-nfs"
        destination = "${NOMAD_ALLOC_DIR}/web-nfs"
      }
      config {
        image   = "python:slim"
        command = "/bin/bash"
        args    = ["-x", "local/entrypoint.sh"]
        ports   = ["http"]
      }
      # this entrypoint writes `date` to index.html only on the first run,
      # to demonstrate that state is persisted in NFS across restarts, etc.
      # afterwards, this can also be seen on the host machine in
      #   /srv/host-nfs/v/my-nfs/index.html
      # or in the other locations node plugin mounts on the host
      #   $ grep my-nfs /proc/mounts
      template {
        destination = "local/entrypoint.sh"
        data        = <<EOF
#!/bin/bash
dir="${NOMAD_ALLOC_DIR}/web-nfs"
test -f $dir/index.html || echo $(date) > $dir/index.html
python -m http.server ${NOMAD_PORT_http} --directory=$dir
EOF
      }
    }
  }
}
