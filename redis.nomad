job "redis" {
  datacenters = ["dc1"]

  group "redis" {
    task "webservice" {
      driver = "docker"

      config {
        image = "redis:3.2"

        labels {
          group = "webservice-cache"
        }
      }
    }
  }
}
