# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

consul {
  address             = "127.0.0.1:8500"
  token               = "${token}"
  client_service_name = "${client_service_name}"
  server_service_name = "${server_service_name}"
}
