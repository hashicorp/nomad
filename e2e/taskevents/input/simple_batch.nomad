job "simple_batch" {
  type         = "batch"
  datacenters  = ["dc1"]

  task "simple_batch" {
    driver = "raw_exec"
    config {
      command = "sleep"
      args    = ["1"]
    }
  }
}
