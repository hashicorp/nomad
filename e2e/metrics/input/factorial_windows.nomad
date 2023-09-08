# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "factorial_windows" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "windows"
  }

  group "test" {
    count = 1

    task "test1" {
      driver = "raw_exec"

      template {
        data = <<EOH
foreach ($loopnumber in 1..2147483647) {
  $result=1;foreach ($number in 1..2147483647) {
    $result = $result * $number
  };$result
}
  EOH

        destination = "local/factorial.ps1"
      }

      config {
        command = "powershell"
        args    = ["local/factorial.ps1"]
      }
    }
  }
}
