# Prometheus Metrics

This configuration file can be used to set up a Prometheus instance to read
data from  Nomad cluster, using Consul for service discovery.

Requirements:
  - See Prometheus's
    [Getting Started](https://prometheus.io/docs/introduction/getting_started/)
    guide for instructions on how to set up a Prometheus server.
  - A running Consul server. This configuration is written for a local remote
    Consul agent, and will need to be updated in the case Consul agent is on a
    remote server.
