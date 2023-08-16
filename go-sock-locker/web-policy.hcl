namespace "default" {
  variables {
    path "locks/*" {
      capabilities = ["read", "list"]
    }
  }
}
