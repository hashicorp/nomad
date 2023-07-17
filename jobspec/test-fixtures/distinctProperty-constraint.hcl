job "foo" {
  constraint {
    distinct_property = "${meta.rack}"
  }
}
