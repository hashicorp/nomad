---
layout: "guides"
page_title: "Monitoring and Alerting"
sidebar_current: "guides-operations-monitoring"
description: |-
  It is possible to enable telemetry on Nomad servers and clients. Nomad 
  can integrate with various metrics dashboards such as Prometheus, Grafana,
  Graphite, DataDog, and Circonus.
---

# Monitoring and Alerting

Nomad provides the opportunity to integrate with metrics dashboard tools such
as [Prometheus](https://prometheus.io/), [Grafana](https://grafana.com/),
[Graphite](https://graphiteapp.org/), [DataDog](https://www.datadoghq.com/),
and [Circonus](https://www.circonus.com).

- [Prometheus](/guides/operations/monitoring-and-alerting/prometheus-metrics.html)

Please refer to the specific documentation links above or in the sidebar for more detailed information about using specific tools to collect metrics on Nomad.
See Nomad's [Metrics API](/api/metrics.html) for more information on how
data can be exposed for other metrics tools as well.
