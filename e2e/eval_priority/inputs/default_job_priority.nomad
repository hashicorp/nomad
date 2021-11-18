job "networking" {
  datacenters = ["dc1", "dc2"]
  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }
  group "bridged" {
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
