# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "java" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }


  group "group" {
    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }


    task "build" {
      lifecycle {
        hook    = "prestart"
        sidecar = false
      }

      driver = "exec"
      config {
        command = "javac"
        args    = ["-d", "${NOMAD_ALLOC_DIR}", "local/Pid.java"]
      }

      template {
        destination = "local/Pid.java"
        data        = <<EOH
public class Pid {
    public static void main(String... s) throws Exception {
        System.out.println("my pid is " + ProcessHandle.current().pid());
    }
}
EOH
      }

      resources {
        cpu    = 50
        memory = 64
      }
    }

    task "java" {
      driver = "java"

      config {
        class_path = "${NOMAD_ALLOC_DIR}"
        class      = "Pid"
      }

      resources {
        cpu    = 50
        memory = 64
      }
    }
  }
}
