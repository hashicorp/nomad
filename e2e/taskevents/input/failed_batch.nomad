job "failed_batch" {
  type         = "batch"
  datacenters  = ["dc1"]

  group "failed_batch" {
    restart {
      attempts = 0
    }
  
    task "failed_batch" {
      driver = "raw_exec"
      config {
        command = "SomeInvalidCommand"
      }
    }
  }
}
