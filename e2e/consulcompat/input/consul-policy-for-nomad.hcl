# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Policy for the Nomad agent. Note that with this policy we must use Workload
# Identity for Connect jobs, or we'll get "failed to derive SI token" errors
# from the client because the Nomad agent's token doesn't have "acl:write"

# The operator:write permission is required for creating config entries for
# connect ingress gateways. operator ACLs are not namespaced, though the
# config entries they can generate are.
operator = "write"

agent_prefix "" {
  policy = "read"
}

node_prefix "" {
  policy = "read"
}

service_prefix "nomad" {
  policy = "write"
}

# for use with Consul ENT
namespace_prefix "" {
  key_prefix "" {
    policy = "read"
  }

  node_prefix "" {
    policy = "read"
  }

  service_prefix "nomad" {
    policy = "write"
  }
}
