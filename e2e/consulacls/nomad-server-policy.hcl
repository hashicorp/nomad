// The operator=write permission is required for creating config entries for
// connect ingress gateways. operator ACLs are not namespaced, though the
// config entries they can generate are.
operator = "write"

namespace_prefix "" {
  // The acl=write permission is required for generating Consul Service Identity
  // tokens for consul connect services. Those services could be configured for
  // any Consul namespace the job-submitter has access to.
  acl = "write"
}

service_prefix "" {
  policy = "write"
}

agent_prefix "" {
  policy = "read"
}

node_prefix "" {
  policy = "read"
}