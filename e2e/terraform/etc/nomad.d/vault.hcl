# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

vault {
  enabled          = true
  address          = "${url}"
  task_token_ttl   = "1h"
  create_from_role = "nomad-tasks"
  namespace        = "${namespace}"
  token            = "${token}"
}
