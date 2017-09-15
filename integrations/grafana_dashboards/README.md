# Setting up a Grafana dashboard for Nomad

This is a sample Grafana dashboard to use for a Nomad cluster.

Requirements:

1. Set up a Prometheus server configured to read data from Nomad. See
  this [sample Prometheus configuration][https://github.com/hashicorp/nomad/integrations/prometheus]
  for an example.

2. Set up a Grafana server configured with a Prometheus data source. See
[Prometheus's instructions for Grafana](https://prometheus.io/docs/visualization/grafana/#creating-a-prometheus-data-source)
for more information

3. [Import](http://docs.grafana.org/features/export_import/) this Grafana
dashboard as a new Grafana dashboard. Make sure the data source name matches
the Prometheus data source in #2.
