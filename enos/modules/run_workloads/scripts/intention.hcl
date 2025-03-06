# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

Kind = "service-intentions"
Name = "count-api"
Sources = [
  {
    Name   = "count-dashboard"
    Action = "allow"
  }
]
