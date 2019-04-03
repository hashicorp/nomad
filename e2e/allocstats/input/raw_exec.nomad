job "test_raw" {
  datacenters = ["dc1"]
  type        = "service"

  group "test" {
   count = 1

    task "test1" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "var=10000;while true; do a=$(awk -v x=$var 'BEGIN{print sqrt(x)}'); ((var++)); done" ]
      } 
    }
}
}
