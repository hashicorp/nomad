# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "identity" {
  datacenters = ["dc1"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "identity" {

    # none task should log no secrets
    task "none" {
      driver = "docker"
      config {
        image = "bash:5"

        #HACK(schmichael) without the ending `sleep 2` we seem to sometimes miss logs :(
        args = ["-c", "wc -c < secrets/nomad_token; env | grep NOMAD_TOKEN; echo done; sleep 2"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    # empty task should log no secrets
    task "empty" {

      identity {}

      driver = "docker"
      config {
        image = "bash:5"

        #HACK(schmichael) without the ending `sleep 2` we seem to sometimes miss logs :(
        args = ["-c", "wc -c < secrets/nomad_token; env | grep NOMAD_TOKEN; echo done; sleep 2"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    # env task should log only env var
    task "env" {

      identity {
        env  = true
        file = false
      }

      driver = "docker"
      config {
        image = "bash:5"

        #HACK(schmichael) without the ending `sleep 2` we seem to sometimes miss logs :(
        args = ["-c", "wc -c < secrets/nomad_token; env | grep NOMAD_TOKEN; echo done; sleep 2"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    # file task should log only env var
    task "file" {

      identity {
        file = true
      }

      driver = "docker"
      config {
        image = "bash:5"

        #HACK(schmichael) without the ending `sleep 2` we seem to sometimes miss logs :(
        args = ["-c", "wc -c < secrets/nomad_token; env | grep NOMAD_TOKEN; echo done; sleep 2"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    task "filepath" {

      identity {
        file     = true
        filepath = "local/nomad_token"
      }

      driver = "docker"
      config {
        image = "bash:5"

        #HACK without the ending `sleep 2` we seem to sometimes miss logs :(
        args = ["-c", "wc -c < local/nomad_token; env | grep NOMAD_TOKEN; echo done; sleep 2"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }

    # falsey task should be the same as no identity block
    task "falsey" {

      identity {
        env  = false
        file = false
      }

      driver = "docker"
      config {
        image = "bash:5"

        #HACK(schmichael) without the ending `sleep 2` we seem to sometimes miss logs :(
        args = ["-c", "wc -c < secrets/nomad_token; env | grep NOMAD_TOKEN; echo done; sleep 2"]
      }
      resources {
        cpu    = 16
        memory = 32
        disk   = 64
      }
    }
  }
}
