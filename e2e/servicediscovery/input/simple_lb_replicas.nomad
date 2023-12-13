# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "simple_lb_replicas" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "db_replica_1" {
    network {
      mode = "bridge"
      port "db_port" {}
    }
    service {
      name     = "db"
      tags     = ["r1"]
      port     = "db_port"
      provider = "nomad"
    }
    task "db" {
      driver = "raw_exec"
      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
      resources {
        cpu    = 10
        memory = 16
      }
    }
  }

  group "db_replica_2" {
    network {
      mode = "bridge"
      port "db_port" {}
    }
    service {
      name     = "db"
      tags     = ["r2"]
      port     = "db_port"
      provider = "nomad"
    }
    task "db" {
      driver = "raw_exec"
      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
      resources {
        cpu    = 10
        memory = 16
      }
    }
  }

  group "db_replica_3" {
    network {
      mode = "bridge"
      port "db_port" {}
    }
    service {
      name     = "db"
      tags     = ["r3"]
      port     = "db_port"
      provider = "nomad"
    }
    task "db" {
      driver = "raw_exec"
      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
      resources {
        cpu    = 10
        memory = 16
      }
    }
  }
}
