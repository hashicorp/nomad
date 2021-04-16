job "template_kv" {
  datacenters = ["dc1"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group-b" {

    consul {
      namespace = "banana"
    }

    task "task-b" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "cat"
        args    = ["local/a.txt"]
      }

      template {
        data        = "value: {{ key \"ns-kv-example\" }}"
        destination = "local/a.txt"
      }
    }
  }

  group "group-z" {

    # no consul namespace set

    task "task-z" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "cat"
        args    = ["local/a.txt"]
      }

      template {
        data        = "value: {{ key \"ns-kv-example\" }}"
        destination = "local/a.txt"
      }
    }
  }
}