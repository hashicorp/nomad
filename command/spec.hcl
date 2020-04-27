name = "default-quota"

description = "Limit the shared default namespace"

# Create a limit for the global region. Additional limits may
# be specified in-order to limit other regions.
limit {
  region = "global"

  region_limit {
    cpu    = 2500
    memory = 1000

    network {
      mbits = 50
    }
  }
}
