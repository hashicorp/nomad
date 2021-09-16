job "networking" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "bridged" {
    network {
      hostname = "mylittlepony-${NOMAD_ALLOC_INDEX}"
      mode     = "bridge"
    }

    task "sleep" {
      driver = "docker"
      config {
        image   = "busybox:1"
        command = "/bin/sleep"
        args    = ["300"]
      }
    }
  }
}
