# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "test3" {

  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  type = "service"

  group "t3" {
    count = 3

    task "t3" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "a=`cksum <<< \"${NOMAD_ALLOC_ID}\"| cut -d ' ' -f1`; if ! (( a % 2 )); then sleep 5000; else exit -1; fi"]
      }
    }

    restart {
      attempts = 0
      delay    = "0s"
      mode     = "fail"
    }

    reschedule {
      attempts  = 2
      interval  = "5m"
      unlimited = false
    }
  }
}
