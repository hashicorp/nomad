job "consul-namespace" {
  group "group" {
    consul {
      namespace = "foo"
    }
  }
}
