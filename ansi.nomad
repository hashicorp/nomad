job "ansi-test" {
  datacenters = ["dc1"]
  type = "service"
  
  group "ansi-test" {
    count = 1
    
    task "ansi-test" {
      driver = "raw_exec"

      config {
        command = "node"
        args = [ "/Users/michael/work/ansi-test/index.js" ]
      }
    }
  }
}
