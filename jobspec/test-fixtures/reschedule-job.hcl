job "foo" {
  datacenters = ["dc1"]
  type = "batch"
  reschedule {
      attempts = 15
      interval = "30m"
  }
  group "bar" {
    count = 3
    task "bar" {
      driver = "raw_exec"
      config {
         command = "bash"
         args    = ["-c", "echo hi"]
      }
    }
  }
}
