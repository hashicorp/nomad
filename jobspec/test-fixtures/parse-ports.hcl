job "parse-ports" {
  group "group" {
    network {
      port "static" {
        static = 9000
      }

      port "dynamic" {}
    }
  }
}
