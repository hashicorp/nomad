job "simple_batch" {
  type        = "batch"
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  task "simple_batch" {
    driver = "raw_exec"

    config {
      command = "sleep"
      args    = ["1"]
    }
  }
}
