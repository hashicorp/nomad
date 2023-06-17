node_pool "example" {

  description = "Example node pool"

  # meta is optional metadata on the node pool, defined as key-value pairs.
  # The scheduler does not use node pool metadata as part of scheduling.
  meta {
    environment = "prod"
    owner       = "sre"
  }

  # The scheduler configuration options specific to this node pool. This block
  # supports a subset of the fields supported in the global scheduler
  # configuration as described at:
  # https://developer.hashicorp.com/nomad/docs/commands/operator/scheduler/set-config
  #
  # * scheduler_algorithm is the scheduling algorithm to use for the pool.
  #   If not defined, the global cluster scheduling algorithm is used.
  #
  # * memory_oversubscription_enabled specifies whether memory oversubscription
  #   is enabled. If not defined, the global cluster configuration is used.
  #
  # Available only in Nomad Enterprise.

  # scheduler_config {
  #   scheduler_algorithm             = "spread"
  #   memory_oversubscription_enabled = true
  # }
}
