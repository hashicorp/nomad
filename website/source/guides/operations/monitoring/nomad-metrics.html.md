---
layout: "guides"
page_title: "Setting up Nomad with Grafana and Prometheus Metrics"
sidebar_current: "guides-operations-monitoring-grafana"
description: |-
  It is possible to collect metrics on Nomad and create dashboards with Grafana
  and Prometheus. Nomad has default configurations for these, but it is
  possible to build and customize these.
---

# Setting up Nomad with Grafana and Prometheus Metrics

Often aggregating and displaying metrics in dashboards can lead to more useful
insights about a cluster. It is easy to get lost in a sea of logs!

This guide explains how to set up configuration for Prometheus and Grafana to
integrate with a Nomad cluster. While this introduces the basics to get a
dashboard up and running, Nomad exposes a wide variety of metrics, which can be
explored via both Grafana and Prometheus.

## What metrics tools can be integrated with Nomad?

Nomad provides the opportunity to integrate with metrics dashboard tools such
as [Prometheus](https://prometheus.io/), [Grafana](https://grafana.com/),
[Graphite](https://graphiteapp.org/), [DataDog](https://www.datadoghq.com/),
and [Circonus](https://www.circonus.com ).

See Nomad's [Metrics API](/api/metrics.html) for more information on how
data can be exposed for other metrics tools as well.

## Setting up metrics

Configurations for Grafana and Prometheus can be found in the
[integrations](https://github.com/hashicorp/nomad/tree/master/integrations) subfolder.

For Prometheus, first follow Prometheus's [Getting Started
Guide](https://prometheus.io/docs/introduction/getting_started/) in order to
set up a Prometheus server. Next, use the [Nomad Prometheus
Configuration](https://github.com/hashicorp/nomad/tree/master/integrations/prometheus/prometheus.yml)
in order to configure Prometheus to talk to a Consul agent to fetch information
about the Nomad cluster. See the
[README](https://github.com/hashicorp/nomad/tree/master/integrations/prometheus/README.md)
for more information.

For Grafana, follow Grafana's [Getting
Started](http://docs.grafana.org/guides/getting_started/) guide to set up a
running Grafana instance. Then, import the sample [Nomad
Dashboard](https://github.com/hashicorp/nomad/tree/master/integrations/grafana/sample_dashboard.json)
for an example Grafana dashboard. This dashboard requires a Prometheus data
source to be configured, see the
[README.md](https://github.com/hashicorp/nomad/tree/master/integrations/grafana/README.md)
for more information.

## Tagged Metrics

As of version 0.7, Nomad will start emitting metrics in a tagged format. Each
metrics can support more than one tag, meaning that it is possible to do a
match over metrics for datapoints such as a particular datacenter, and return
all metrics with this tag.

See how [Grafana](http://docs.grafana.org/v3.1/reference/templating/) enables
tagged metrics easily.

