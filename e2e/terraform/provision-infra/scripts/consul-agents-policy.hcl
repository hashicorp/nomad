# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

# Consul agents only need to register themselves and read services

node_prefix "" {
  policy = "write"
}

service_prefix "" {
  policy = "read"
}
