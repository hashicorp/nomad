server {
  enabled          = true
  bootstrap_expect = 3

  default_scheduler_config {
    # Set a default carbon score in case not all nodes fingerprint it
    carbon_default_score = 50

    # These weights will enable carbon scoring and prioritize it over
    # binpacking but less than job-anti-affinity
    scoring_weights = {
      "job-anti-affinity" = 2
      carbon              = 1.5
      binpack             = 0.5
    }
  }
}
