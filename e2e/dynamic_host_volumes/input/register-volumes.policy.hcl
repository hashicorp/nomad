// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

namespace "*" {
  policy = "write"
  capabilities = [
    "host-volume-register",
  ]
}

agent {
  policy = "read"
}

operator {
  policy = "read"
}

node {
  policy = "read"
}

node_pool "*" {
  policy = "read"
}
