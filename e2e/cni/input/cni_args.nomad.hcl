# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "cni_args" {
  group "group" {
    network {
      mode = "cni/cni_args"
      cni {
        # feature under test
        args = {
          # the message gets placed as a file called "victory" in the task dir
          # specified here by the cni_args.sh plugin. Using node pool allows us
          # to test interpolation as an extra.
          FancyMessage = "${node.pool}"
          FancyTaskDir = "${NOMAD_ALLOC_DIR}/task/local"
        }
      }
    }
    task "task" {
      driver = "docker"
      config {
        image   = "busybox:1"
        command = "sh"
        args    = ["-c", "cat local/victory; sleep 60"]
      }
    }
    # go faster
    update {
      min_healthy_time = "0s"
    }
    # fail faster (if it does fail)
    reschedule {
      attempts  = 0
      unlimited = false
    }
    restart {
      attempts = 0
      mode     = "fail"
    }
  }
}
