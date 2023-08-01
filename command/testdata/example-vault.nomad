job "vault" {
  datacenters = ["dc1"]
  group "group" {
    task "task" {
      driver = "docker"
      config {
        image = "alpine:latest"
      }
      vault {
        policies = ["my-policy"]
      }
    }
  }
}
