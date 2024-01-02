# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "java_pid" {
  datacenters = ["dc1"]
  type        = "batch"

  group "java" {

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
    }

    task "pid" {
      driver = "java"
      config {
        class_path = "${NOMAD_ALLOC_DIR}"
        class      = "Pid"
        pid_mode   = "host"
        ipc_mode   = "host"
      }
    }
  }
}