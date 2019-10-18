# Additional client configuration overlay to support testing
# affinity, constraints, and spread.

datacenter = "dc2"

client {
  meta {
    "rack" = "r2"
  }
}
