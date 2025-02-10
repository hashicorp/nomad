# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

server {
  enabled          = true
  bootstrap_expect = 3
}

keyring "awskms" {
  active     = true
  region     = "${aws_region}"
  kms_key_id = "${aws_kms_key_id}"
}

keyring "aead" {
  active = false
}
