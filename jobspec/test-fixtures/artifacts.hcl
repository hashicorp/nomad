# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "binstore-storagelocker" {
  group "binsl" {
    task "binstore" {
      driver = "docker"

      artifact {
        source      = "http://foo.com/bar"
        destination = ""

        options {
          foo = "bar"
        }
      }

      artifact {
        source = "http://foo.com/baz"
      }

      artifact {
        source      = "http://foo.com/bam"
        destination = "var/foo"
      }

      artifact {
        source = "https://example.com/file.txt"

        headers {
          User-Agent    = "nomad"
          X-Nomad-Alloc = "alloc"
        }
      }
    }
  }
}
