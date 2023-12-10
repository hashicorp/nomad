# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

// The Nomad Client will be registering things into its buddy Consul Client.
// Note: because we also test the use of Consul namespaces, this token must be
// able to register services, read the keystore, and read node data for any
// namespace.
// The operator=write permission is required for creating config entries for
// connect ingress gateways. operator ACLs are not namespaced, though the
// config entries they can generate are.
operator = "write"

agent_prefix "" {
  policy = "read"
}

namespace_prefix "" {
  // The acl=write permission is required for generating Consul Service Identity
  // tokens for consul connect services. Those services could be configured for
  // any Consul namespace the job-submitter has access to.
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
