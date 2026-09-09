job "group-selection" {
  type = "service"

  group_selection "runtime" {
    count  = 2
    groups = ["encoder", "orin", "thor"]
  }

  group "encoder" {
    count = 2
    constraint {
      attribute = "${meta.profile}"
      value     = "encoder"
    }
    task "service" {
      driver = "raw_exec"
      config {
        command = "/bin/sleep"
        args    = ["600"]
      }
      resources {
        cpu    = 1000
        memory = 1000
      }
    }
  }

  group "orin" {
    count = 3
    constraint {
      attribute = "${meta.profile}"
      value     = "orin"
    }
    task "service" {
      driver = "raw_exec"
      config {
        command = "/bin/sleep"
        args    = ["600"]
      }
      resources {
        cpu    = 800
        memory = 800
      }
    }
  }

  group "thor" {
    count = 5
    constraint {
      attribute = "${meta.profile}"
      value     = "thor"
    }
    task "service" {
      driver = "raw_exec"
      config {
        command = "/bin/sleep"
        args    = ["600"]
      }
      resources {
        cpu    = 500
        memory = 500
      }
    }
  }
}
