# Sleeper is a fake service that outputs its pid to a file named `pid` to
# assert duplicate tasks are never started.

job "sleeper" {
  datacenters = ["dc1"]
  task "sleeper" {
    driver = "raw_exec"
    config {
      command = "/bin/bash"
      args    = ["-c", "echo $$ >> pid && sleep 999999"]
    }
  }
}
