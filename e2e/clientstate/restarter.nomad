# Restarter fakes being a flaky service that crashes and restarts constantly.
# Restarting the Nomad agent during task restarts was a known cause of state
# corruption in v0.8.

job "restarter" {
  datacenters = ["dc1"]
  group "restarter" {
    restart {
      attempts = 100
      delay    = "3s"
    }
    task "restarter" {
      driver = "raw_exec"
      config {
        command = "/bin/bash"
        args    = ["-c", "echo $$ >> pid && sleep 1 && exit 99"]
      }
    }
  }
}
