# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "parameterized_job" {
  parameterized {
    payload       = "required"
    meta_required = ["foo", "bar"]
    meta_optional = ["baz", "bam"]
  }

  group "foo" {
    task "bar" {
      driver = "docker"

      dispatch_payload {
        file = "foo/bar"
      }
    }
  }
}
