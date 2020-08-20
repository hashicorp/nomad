//e2e:service script=validate.sh
job "networking" {
  datacenters = ["nick-east-1"]
  group "basic" {
    network {
      mode = "bridge"
    }

    task "sleep" {
      driver = "docker"
      config {
        image   = "busybox:1"
        command = "/bin/sleep"
        args    = ["5"]
      }
    }
  }
}