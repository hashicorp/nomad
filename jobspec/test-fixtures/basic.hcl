job "binstore-storagelocker" {
  region      = "fooregion"
  type        = "batch"
  priority    = 52
  all_at_once = true
  datacenters = ["us2", "eu1"]
  vault_token = "foo"

  meta {
    foo = "bar"
  }

  constraint {
    attribute = "kernel.os"
    value     = "windows"
  }

  update {
    stagger      = "60s"
    max_parallel = 2
    health_check = "manual"
    min_healthy_time = "10s"
    healthy_deadline = "10m"
    auto_revert = true
    canary = 1
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

    restart {
      attempts = 5
      interval = "10m"
      delay    = "15s"
      mode     = "delay"
    }

    ephemeral_disk {
        sticky = true
        size = 150
    }

    update {
        max_parallel = 3
        health_check = "checks"
        min_healthy_time = "1s"
        healthy_deadline = "1m"
        auto_revert = false
        canary = 2
    }

    task "binstore" {
      driver = "docker"
      user   = "bob"
      leader = true

      config {
        image = "hashicorp/binstore"

        labels {
          FOO = "bar"
        }
      }

      logs {
        max_files     = 14
        max_file_size = 101
      }

      env {
        HELLO = "world"
        LOREM = "ipsum"
      }

      service {
        tags = ["foo", "bar"]
        port = "http"

        check {
          name     = "check-name"
          type     = "tcp"
          interval = "10s"
          timeout  = "2s"
          port     = "admin"
        }
      }

      resources {
        cpu    = 500
        memory = 128

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

          port "http" {
          }

          port "https" {
          }

          port "admin" {
          }
        }
      }

      kill_timeout = "22s"

      artifact {
        source = "http://foo.com/artifact"

        options {
          checksum = "md5:b8a4f3f72ecab0510a6a31e997461c5f"
        }
      }

      artifact {
        source = "http://bar.com/artifact"
        destination = "test/foo/"
        mode = "file"

        options {
          checksum = "md5:ff1cc0d3432dad54d607c1505fb7245c"
        }
      }

      vault {
        policies = ["foo", "bar"]
      }

      template {
        source = "foo"
        destination = "foo"
        change_mode = "foo"
        change_signal = "foo"
        splay = "10s"
        env = true
      }

      template {
        source = "bar"
        destination = "bar"
        perms = "777"
        left_delimiter = "--"
        right_delimiter = "__"
      }
    }

    task "storagelocker" {
      driver = "docker"

      config {
        image = "hashicorp/storagelocker"
      }

      resources {
        cpu    = 500
        memory = 128
        iops   = 30
      }

      constraint {
        attribute = "kernel.arch"
        value     = "amd64"
      }

      vault {
        policies = ["foo", "bar"]
        env = false
        change_mode = "signal"
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
