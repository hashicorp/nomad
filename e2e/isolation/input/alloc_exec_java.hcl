# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "java_exec" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    update {
      min_healthy_time = "2s"
    }

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
        args    = ["-d", "${NOMAD_ALLOC_DIR}", "local/Sleep.java"]
      }

      template {
        destination = "local/Sleep.java"
        data        = <<EOH
public class Sleep {
    public static void main(String... s) throws Exception {
        Thread.sleep(999999999);
    }
}
EOH
      }

      resources {
        cpu    = 50
        memory = 64
      }
    }

    task "sleep" {
      driver = "java"

      config {
        class_path = "${NOMAD_ALLOC_DIR}"
        class      = "Sleep"
      }

      resources {
        cpu    = 50
        memory = 64
      }
    }
  }
}
