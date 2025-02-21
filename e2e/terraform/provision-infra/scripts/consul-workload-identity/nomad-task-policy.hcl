// A policy used by the Consul role associated
// with nomad tasks.
// Used for Workload Identity integration.

service_prefix "" {
  policy = "read"
}

key_prefix "" {
  policy = "read"
}
