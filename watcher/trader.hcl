job "trader-solo" {
    meta {
        START_AT = "15:58:00"
        END_AT = "15:59:00"
    }

    group "g" {
        task "trading" {
            driver = "raw_exec"

            config {
                command = "watch"
                args    = ["-t", "date"]
            }
        }

        task "schedule-blocker" {
            driver = "raw_exec"

            lifecycle {
                hook = "prestart"
                sidecar = false
            }

            config {
                command = "/Users/mike/code/nomad/watcher/nomad-watcher"
                args    = ["block-local", "${NOMAD_META_START_AT}", "${NOMAD_META_END_AT}"]
            }
        }
    }
}
