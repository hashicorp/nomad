job "services_group" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group-b" {

    consul {
      namespace = "banana"
    }

    network {
      mode = "bridge"
      port "port-b" {
        to = 1234
      }
    }

    service {
      name = "b1"
      port = "port-b"

      check {
        name     = "ping-b1"
        type     = "tcp"
        interval = "10s"
        timeout  = "2s"
      }
    }

    service {
      name = "b2"
      port = "port-b"

      check {
        name     = "ping-b2"
        type     = "tcp"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "task-b" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "nc"
        args    = ["-ll", "-p", "1234", "-e", "/bin/cat"]
      }
    }
  }

  group "group-c" {

    consul {
      namespace = "cherry"
    }

    network {
      mode = "bridge"
      port "port-c" {
        to = 1234
      }
    }

    service {
      name = "c1"
      port = "port-c"

      check {
        name     = "ping-c1"
        type     = "tcp"
        interval = "10s"
        timeout  = "2s"
      }
    }

    service {
      name = "c2"
      port = "port-c"

      check {
        name     = "ping-c2"
        type     = "tcp"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "task-c" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "nc"
        args    = ["-ll", "-p", "1234", "-e", "/bin/cat"]
      }
    }
  }

  group "group-z" {

    # consul namespace not set

    network {
      mode = "bridge"
      port "port-z" {
        to = 1234
      }
    }

    service {
      name = "z1"
      port = "port-z"

      check {
        name     = "ping-z1"
        type     = "tcp"
        interval = "10s"
        timeout  = "2s"
      }
    }

    service {
      name = "z2"
      port = "port-z"

      check {
        name     = "ping-z2"
        type     = "tcp"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "task-z" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "nc"
        args    = ["-ll", "-p", "1234", "-e", "/bin/cat"]
      }
    }
  }
}
