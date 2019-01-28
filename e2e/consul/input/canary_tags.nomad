job "consul_canary_test" {
  datacenters = ["dc1"]

  group "consul_canary_test" {
    count = 2

    task "consul_canary_test" {
      driver = "mock_driver"

      config {
        run_for   = "10m"
        exit_code = 9
      }

      service {
        name = "canarytest"
        tags = ["foo", "bar"]
        canary_tags = ["foo", "canary"]
      }
    }

    update {
      max_parallel     = 1
      canary           = 1
      min_healthy_time = "1s"
      health_check     = "task_states"
      auto_revert      = false
    }

    restart {
      attempts = 0
      delay    = "0s"
      mode     = "fail"
    }
  }
}
