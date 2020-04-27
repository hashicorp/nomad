job "example" {
  vault {
    policies = ["job"]
  }

  group "cache" {
    vault {
      policies = ["group"]
    }

    task "redis" {}

    task "redis2" {
      vault {
        policies = ["task"]
        env      = false
      }
    }
  }

  group "cache2" {
    task "redis" {}
  }
}
