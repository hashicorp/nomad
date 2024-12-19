# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# TODO: because Nomad should own most of these interactions, I think
# it might be possible to reduce this to:
#
# node_prefix "" {
#   policy = write
# }

acl = "write"

agent_prefix "" {
  policy = "write"
}

event_prefix "" {
  policy = "write"
}

key_prefix "" {
  policy = "write"
}

node_prefix "" {
  policy = "write"
}

query_prefix "" {
  policy = "write"
}

service_prefix "" {
  policy = "write"
}
