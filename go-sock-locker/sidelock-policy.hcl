namespace "default" {
  variables {
    path "nomad/jobs/sidelock/*" {
      capabilities = ["read", "write", "list", "destroy"]
    }
  }
}
