job "group_service_check_script" {
  group "group" {
    count = 1

    network {
      mode = "bridge"

      port "http" {
        static = 80
        to     = 8080
      }
    }

    service {
      name = "foo-service"
      port = "http"

      check {
        name           = "check-name"
        type           = "script"
        command        = "/bin/true"
        interval       = "10s"
        timeout        = "2s"
        initial_status = "passing"
        task           = "foo"
      }
    }

    task "foo" {}
  }
}
