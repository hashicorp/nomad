# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

server {
  enabled          = true
  bootstrap_expect = 3
}

acl {
  enabled = true

  # These values are used by the testACLTokenExpiration test within the acl
  # test suite. If these need to be updated, please ensure the new values are
  # reflected within the test suite and do not break the tests. Thanks.
  token_min_expiration_ttl = "1s"
  token_max_expiration_ttl = "24h"
}
