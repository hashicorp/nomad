# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

vault {
  enabled               = true
  address               = "${url}"
  namespace             = "${namespace}"
  jwt_auth_backend_path = "${jwt_auth_backend_path}/"

  default_identity {
    aud = ["vault.io"]
    ttl = "1h"
  }
}
