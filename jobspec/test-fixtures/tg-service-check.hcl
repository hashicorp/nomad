# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


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
      name      = "foo-service"
      port      = "http"
      on_update = "ignore"

      check {
        name           = "check-name"
        type           = "script"
        command        = "/bin/true"
        interval       = "10s"
        timeout        = "2s"
        initial_status = "passing"
        task           = "foo"
        on_update      = "ignore"
        body           = "post body"
      }
    }

    task "foo" {}
  }
}
