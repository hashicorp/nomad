# Prometheus Metrics

This configuration file can be used to set up a Prometheus instance to read
data from  Nomad cluster, using Consul for service discovery.

Requirements:
  - See Prometheus's
    [Getting Started](https://prometheus.io/docs/introduction/getting_started/)
    guide for instructions on how to set up a Prometheus server.
  - A running Consul server, which will need to be added to this configuration
    file.
