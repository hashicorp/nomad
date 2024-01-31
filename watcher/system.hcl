job "schedule-watcher" {
    meta {
        Tick = "4"
    }

    type = "system"
    group "g" {
        task "node-watcher" {
            driver = "raw_exec"

            config {
                command = "/Users/mike/code/nomad/watcher/nomad-watcher"
                args    = ["system"]
            }
        }
    }
}
