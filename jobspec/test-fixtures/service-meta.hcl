job "check_meta" {
    type = "service"
    group "group" {
        count = 1

        task "task" {
          service {
            port = "http"
            meta {
                foo = "bar"
            }
          }
        }
    }
}

