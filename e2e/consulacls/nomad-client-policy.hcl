// The Nomad Client will be registering things into its buddy Consul Client.
// Note: because we also test the use of Consul namespaces, this token must be
// able to register services, read the keystore, and read node data for any
// namespace.

agent_prefix "" {
  policy = "read"
}

namespace_prefix "" {
  key_prefix "" {
    policy = "read"
  }

  node_prefix "" {
    policy = "read"
  }

  service_prefix "" {
    policy = "write"
  }
}