// The Nomad Client will be registering things into its buddy Consul Client.

service_prefix "" {
  policy = "write"
}

agent_prefix "" {
  policy = "read"
}

node_prefix "" {
  policy = "read"
}
