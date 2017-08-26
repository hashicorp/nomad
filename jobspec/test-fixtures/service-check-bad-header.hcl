job "check_bad_header" {
    type = "service"
    group "group" {
        count = 1

        task "task" {
          service {
            tags = ["bar"]
            port = "http"

            check {
              name     = "check-name"
              type     = "http"
              path     = "/"
              method   = "POST"
              interval = "10s"
              timeout  = "2s"
              initial_status = "passing"

              header {
                Authorization = "Should be a []string!"
              }
            }
          }
        }
    }
}

