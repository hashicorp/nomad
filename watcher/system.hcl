job "schedule-watcher" {
  meta {
    Tick = "6"
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
