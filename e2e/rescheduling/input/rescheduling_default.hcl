job "test" {
  datacenters = ["dc1"]
  type        = "service"

  group "t" {
    count = 3

    task "t" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "lol 5000"]
      }
    }

    restart {
      attempts = 0
      delay    = "0s"
      mode     = "fail"
    }
  }
}
