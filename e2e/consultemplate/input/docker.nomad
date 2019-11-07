job "test1" {
  datacenters = ["dc1", "dc2"]
  type        = "service"

  group "test1" {
    count = 1

    task "test" {
      driver = "docker"

      config {
        image = "redis:3.2"
      }

      template {
        data        = "---\nkey: {{ key \"consultemplatetest\" }}"
        destination = "local/file.yml"
        change_mode = "restart"
      }
    }
  }
}
