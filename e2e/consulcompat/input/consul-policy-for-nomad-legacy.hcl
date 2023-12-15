# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Policy for the Nomad agent. Note that this policy will work with Workload
# Identity for Connect jobs, but is more highly-privileged than we need.

# The operator:write permission is required for creating config entries for
# connect ingress gateways. operator ACLs are not namespaced, though the
# config entries they can generate are.
operator = "write"

agent_prefix "" {
  policy = "read"
}

# The acl:write permission is required for minting Consul Service Identity
# tokens for Connect services with Consul CE (which has no namespaces)
acl = "write"

key_prefix "" {
  policy = "read"
}

node_prefix "" {
  policy = "read"
}

service_prefix "" {
  policy = "write"
}

# for use with Consul ENT
namespace_prefix "prod" {

  acl = "write"

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
