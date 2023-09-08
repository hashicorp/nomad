# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "mem_windows" {
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
$mem_stress = @()
for ($i = 0; $i -lt ###; $i++) { $mem_stress += ("a" * 200MB) }
  EOH

        destination = "local/memtest.ps1"
      }

      config {
        command = "powershell"
        args    = ["local/memtest.ps1"]
      }
    }
  }
}
