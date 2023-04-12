data_dir = "/tmp/nomad/data"

server {
  enabled = true

  bootstrap_expect = 1
}

client {
  enabled = true
  host_volume "host-nfs" {
    path      = "/srv/host-nfs"
    read_only = false
  }
}

plugin "docker" {
  config {
    # for node plugin to mount disk.
    allow_privileged = true
  }
}
