job "trader" {
    meta {
        START_AT = "11:02:40"
        END_AT = "11:02:50"
    }

    group "g" {
        task "trader" {
            driver = "raw_exec"

            config {
                command = "/Users/mike/code/nomad/watcher/nomad-watcher"
                args    = ["wait", "watch", "date"]
            }

            env {
                ALLOC_DIR = "${NOMAD_ALLOC_DIR}"
            }
        }

        task "watcher" {
            driver = "raw_exec"
            config {
                command = "/Users/mike/code/nomad/watcher/nomad-watcher"
                args    = ["watch"]
            }

            lifecycle {
                hook = "prestart"
                sidecar = true
            }

            identity {
                env         = true
                change_mode = "restart"
            }

            restart {
                attempts = 100
                delay    = "1s"
                interval = "1s"
                mode     = "fail"
            }

            env {
                START_AT = "${NOMAD_META_START_AT}"
                END_AT = "${NOMAD_META_END_AT}"
                ALLOC_DIR = "${NOMAD_ALLOC_DIR}"
            }
        }
    }
}

# Nomad alloc signal is a client API call
# Look at SIGSTOP, SIGCONT
# Client caches tokens, so disconnected should be okay
    # While disconnected, you should use it 4eva
# (part of )

