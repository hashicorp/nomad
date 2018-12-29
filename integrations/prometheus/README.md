# Prometheus Metrics

This configuration file can be used to set up a Prometheus instance to read
data from  Nomad cluster, using Consul for service discovery.

Requirements:
  - See Prometheus's
    [Getting Started](https://prometheus.io/docs/introduction/getting_started/)
    guide for instructions on how to set up a Prometheus server.
  - This configuration is written assuming there is a local Consul agent being
    run along side the Prometheus server. If this is not the case, the
    configuration will need to be updated to point at the remote Consul agent.
