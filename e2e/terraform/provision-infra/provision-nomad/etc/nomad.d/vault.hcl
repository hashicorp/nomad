# Copyright IBM Corp. 2015, 2025
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
