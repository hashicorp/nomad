// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// policy without namespaces, for Consul CE. This policy is for Nomad tasks
// using WI so they can read services and KV from Consul when rendering templates.

key_prefix "" {
  policy = "read"
}

service_prefix "" {
  policy = "read"
}
