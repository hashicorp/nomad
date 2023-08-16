# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "binstore-storagelocker" {
  region       = "fooregion"
  namespace    = "foonamespace"
  type         = "batch"
  priority     = 52
  all_at_once  = true
  datacenters  = ["us2", "eu1"]
  consul_token = "abc"
  vault_token  = "foo"

  meta {
    foo = "bar"
  }

  constraint {
    attribute = "kernel.os"
    value     = "windows"
  }

  constraint {
    attribute = "${attr.vault.version}"
    value     = ">= 0.6.1"
    operator  = "semver"
  }

  affinity {
    attribute = "${meta.team}"
    value     = "mobile"
    operator  = "="
    weight    = 50
  }

  spread {
    attribute = "${meta.rack}"
    weight    = 100

    target "r1" {
      percent = 40
    }

    target "r2" {
      percent = 60
    }
  }

  update {
    stagger           = "60s"
    max_parallel      = 2
    health_check      = "manual"
    min_healthy_time  = "10s"
    healthy_deadline  = "10m"
    progress_deadline = "10m"
    auto_revert       = true
    auto_promote      = true
    canary            = 1
  }

  task "outside" {
    driver = "java"

    config {
      jar_path = "s3://my-cool-store/foo.jar"
    }

    meta {
      my-cool-key = "foobar"
    }
  }

  group "binsl" {
    count = 5

    volume "foo" {
      type   = "host"
      source = "/path"
    }

    volume "bar" {
      type            = "csi"
      source          = "bar-vol"
      read_only       = true
      attachment_mode = "file-system"
      access_mode     = "single-mode-writer"

      mount_options {
        fs_type = "ext4"
      }
    }

    volume "baz" {
      type   = "csi"
      source = "bar-vol"

      mount_options {
        mount_flags = ["ro"]
      }

      per_alloc = true
    }

    restart {
      attempts         = 5
      interval         = "10m"
      delay            = "15s"
      mode             = "delay"
      render_templates = false
    }

    reschedule {
      attempts = 5
      interval = "12h"
    }

    ephemeral_disk {
      sticky = true
      size   = 150
    }

    update {
      max_parallel      = 3
      health_check      = "checks"
      min_healthy_time  = "1s"
      healthy_deadline  = "1m"
      progress_deadline = "1m"
      auto_revert       = false
      auto_promote      = false
      canary            = 2
    }

    migrate {
      max_parallel     = 2
      health_check     = "task_states"
      min_healthy_time = "11s"
      healthy_deadline = "11m"
    }

    affinity {
      attribute = "${node.datacenter}"
      value     = "dc2"
      operator  = "="
      weight    = 100
    }

    spread {
      attribute = "${node.datacenter}"
      weight    = 50

      target "dc1" {
        percent = 50
      }

      target "dc2" {
        percent = 25
      }

      target "dc3" {
        percent = 25
      }
    }

    stop_after_client_disconnect = "120s"
    max_client_disconnect        = "120h"

    task "binstore" {
      driver = "docker"
      user   = "bob"
      leader = true
      kind   = "connect-proxy:test"

      affinity {
        attribute = "${meta.foo}"
        value     = "a,b,c"
        operator  = "set_contains"
        weight    = 25
      }

      config {
        image = "hashicorp/binstore"

        labels {
          FOO = "bar"
        }
      }

      volume_mount {
        volume      = "foo"
        destination = "/mnt/foo"
      }

      restart {
        attempts = 10
      }

      logs {
        disabled      = false
        max_files     = 14
        max_file_size = 101
      }

      env {
        HELLO = "world"
        LOREM = "ipsum"
      }

      service {
        meta {
          abc = "123"
        }


        canary_meta {
          canary = "boom"
        }

        tags        = ["foo", "bar"]
        canary_tags = ["canary", "bam"]
        port        = "http"

        check {
          name         = "check-name"
          type         = "tcp"
          interval     = "10s"
          timeout      = "2s"
          port         = "admin"
          grpc_service = "foo.Bar"
          grpc_use_tls = true

          check_restart {
            limit           = 3
            grace           = "10s"
            ignore_warnings = true
          }
        }
      }

      resources {
        cpu        = 500
        memory     = 128
        memory_max = 256

        network {
          mbits = "100"

          port "one" {
            static = 1
          }

          port "two" {
            static = 2
          }

          port "three" {
            static = 3
          }

          port "http" {}

          port "https" {}

          port "admin" {}
        }

        device "nvidia/gpu" {
          count = 10

          constraint {
            attribute = "${device.attr.memory}"
            value     = "2GB"
            operator  = ">"
          }

          affinity {
            attribute = "${device.model}"
            value     = "1080ti"
            weight    = 50
          }
        }

        device "intel/gpu" {}
      }

      kill_timeout = "22s"

      shutdown_delay = "11s"

      artifact {
        source = "http://foo.com/artifact"

        options {
          checksum = "md5:b8a4f3f72ecab0510a6a31e997461c5f"
        }
      }

      artifact {
        source      = "http://bar.com/artifact"
        destination = "test/foo/"
        mode        = "file"

        options {
          checksum = "md5:ff1cc0d3432dad54d607c1505fb7245c"
        }
      }

      vault {
        namespace = "ns1"
        policies  = ["foo", "bar"]
      }

      template {
        source               = "foo"
        destination          = "foo"
        change_mode          = "foo"
        change_signal        = "foo"
        splay                = "10s"
        env                  = true
        vault_grace          = "33s"
        error_on_missing_key = true
      }

      template {
        source      = "bar"
        destination = "bar"
        change_mode = "script"
        change_script {
          command       = "/bin/foo"
          args          = ["-debug", "-verbose"]
          timeout       = "5s"
          fail_on_error = false
        }
        perms           = "777"
        uid             = 1001
        gid             = 20
        left_delimiter  = "--"
        right_delimiter = "__"
      }
    }

    task "storagelocker" {
      driver = "docker"

      lifecycle {
        hook    = "prestart"
        sidecar = true
      }

      config {
        image = "hashicorp/storagelocker"
      }

      resources {
        cpu    = 500
        memory = 128
      }

      constraint {
        attribute = "kernel.arch"
        value     = "amd64"
      }

      vault {
        policies      = ["foo", "bar"]
        env           = false
        disable_file  = false
        change_mode   = "signal"
        change_signal = "SIGUSR1"
      }
    }

    constraint {
      attribute = "kernel.os"
      value     = "linux"
    }

    meta {
      elb_mode     = "tcp"
      elb_interval = 10
      elb_checks   = 3
    }
  }
}
