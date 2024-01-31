job "trader-solo" {
    meta {
        START_AT = "22:05:20"
        END_AT = "22:05:40"
    }

    group "g" {
        task "trading" {
            driver = "raw_exec"

            config {
                command = "watch"
                args    = ["-t", "date"]
            }
        }

        task "blocker" {
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

        task "watcher" {
            driver = "raw_exec"

            lifecycle {
                hook = "poststart"
                sidecar = true
            }

            config {
                command = "/Users/mike/code/nomad/watcher/nomad-watcher"
                args    = ["bound", "${NOMAD_META_START_AT}", "${NOMAD_META_END_AT}"]
            }
        }
    }
}
