# From https://github.com/hashicorp/nomad/issues/5770#issuecomment-500023563
job "failed_alloc" {
  datacenters = ["dc1"]
  type = "service"

  group "failed_alloc_group" {

    # Disable restarts
    restart {
      attempts = 0
      mode     = "fail"
    }

    # Disable rescheduling
    reschedule {
      attempts  = 0
      unlimited = false
    }

    task "failed_alloc_task" {
      driver = "docker"
      config {
        image = "does/not/exist"
      }

      service {
          name = "failed-alloc-service"
          port = "http"
      }

      resources {
        network {
          port "http" {}
        }
      }

      # Vault seems to be required.
      vault {
        policies = ["doesnotexist"]
      }
    }
  }
}
