job "multiregion_job" {

  multiregion {

    strategy {
      max_parallel = 1
      on_failure   = "fail_all"
    }

    region "west" {
      count       = 2
      datacenters = ["west-1"]
      meta {
        region_code = "W"
      }
    }

    region "east" {
      count       = 1
      datacenters = ["east-1", "east-2"]
      meta {
        region_code = "E"
      }
    }
  }
}
