// The acl=write permission is required for generating Consul Service Identity
// tokens for consul connect services.
acl = "write"

// The operator=write permission is required for creating config entries for
// connect ingress gateways.
operator = "write"

service_prefix "" {
  policy = "write"
}

agent_prefix "" {
  policy = "read"
}

node_prefix "" {
  policy = "read"
}