---
layout: "guides"
page_title: "Using Prometheus to Monitor Nomad Metrics"
sidebar_current: "guides-monitoring"
description: |-
  It is possible to collect metrics on Nomad with Prometheus after enabling
  telemetry on Nomad servers and clients.
---

# Using Prometheus to Monitor Nomad Metrics

This guide explains how to configure [Prometheus][prometheus]
to integrate with a Nomad cluster. While this guide introduces the basics of
enabling [telemetry][telemetry] and collecting metrics, a Nomad operator can
go much further by customizing dashboards and setting up [alerting][alerting]
as well.

## Reference Material

- [Configuring Prometheus][configuring prometheus]
- [Telemetry Stanza in Nomad Agent Configuration][telemetry stanza]

## Estimated Time to Complete

20 minutes

## Challenge

Think of a scenario where a Nomad operator needs to deploy Prometheus to
collect metrics from a Nomad cluster. The operator must enable telemetry on
the Nomad servers and clients as well as configure Prometheus to use Consul
for service discovery.

## Solution

Deploy Prometheus with a configuration that accounts for a highly dynamic
environment. Integrate service discovery into the configuration file to avoid
using hard-coded IP addresses. Place the Prometheus deployment behind
[fabio][fabio] (this will allow easy access to the Prometheus web interface
by allowing the Nomad operator to hit any of the client nodes at the `/` path.

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this
[repo](https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud)
to easily provision a sandbox environment. This guide will assume a cluster with
one server node and three client nodes.

-> **Please Note:** This guide is for demo purposes and is only using a single server
node. In a production cluster, 3 or 5 server nodes are recommended.

## Steps

### Step 1: Enable Telemetry on Nomad Servers and Clients

Add the stanza below in your Nomad client and server configuration 
files. If you have used the provided repo in this guide to set up a Nomad
cluster, the configuration file will be `/etc/nomad.d/nomad.hcl`.
After making this change, restart the Nomad service on each server and
client node.

```hcl
telemetry {
  collection_interval = "1s"
  disable_hostname = true
  prometheus_metrics = true
  publish_allocation_metrics = true
  publish_node_metrics = true 
}
```

### Step 2: Create a Job for Fabio

Create a job for Fabio and name it `fabio.nomad`

```hcl
job "fabio" {
  datacenters = ["dc1"]
  type = "system"

  group "fabio" {
    task "fabio" {
      driver = "docker"
      config {
        image = "magiconair/fabio"
        network_mode = "host"
      }

      resources {
        cpu    = 100
        memory = 64
        network {
          mbits = 20
          port "lb" {
            static = 9999
          }
          port "ui" {
            static = 9998
          }
        }
      }
    }
  }
}
```
To learn more about fabio and the options used in this job file, see
[Load Balancing with Fabio][fabio-lb]. For the purpose of this guide, it is
important to note that the `type` option is set to [system][system] so that
fabio will be deployed on all client nodes. We have also set `network_mode` to
`host` so that fabio will be able to use Consul for service discovery.

### Step 3: Run the Fabio Job

We can now register our fabio job:

```shell
$ nomad job run fabio.nomad 
==> Monitoring evaluation "7b96701e"
    Evaluation triggered by job "fabio"
    Allocation "d0e34682" created: node "28d7f859", group "fabio"
    Allocation "238ec0f7" created: node "510898b6", group "fabio"
    Allocation "9a2e8359" created: node "f3739267", group "fabio"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "7b96701e" finished with status "complete"
```
At this point, you should be able to visit any one of your client nodes at port
`9998` and see the web interface for fabio. The routing table will be empty
since we have not yet deployed anything that fabio can route to.
Accordingly, if you visit any of the client nodes at port `9999` at this
point, you will get a `404` HTTP response. That will change soon.

### Step 4: Create a Job for Prometheus

Create a job for Prometheus and name it `prometheus.nomad`

```hcl
job "prometheus" {
  datacenters = ["dc1"]
  type = "service"

  update {
    max_parallel = 1
    min_healthy_time = "10s"
    healthy_deadline = "3m"
    auto_revert = false
    canary = 0
  }

  migrate {
    max_parallel = 1
    health_check = "checks"
    min_healthy_time = "10s"
    healthy_deadline = "5m"
  }

  group "monitoring" {
    count = 1
    restart {
      attempts = 2
      interval = "30m"
      delay = "15s"
      mode = "fail"
    }
    ephemeral_disk {
      size = 300
    }

    task "prometheus" {
      template {
        change_mode = "noop"
        destination = "local/prometheus.yml"
        data = <<EOH
---
global:
  scrape_interval:     5s
  evaluation_interval: 5s

scrape_configs:

  - job_name: 'nomad_metrics'

    consul_sd_configs:
    - server: '{{ env "NOMAD_IP_prometheus_ui" }}:8500'
      services: ['nomad-client', 'nomad']

    relabel_configs:
    - source_labels: ['__meta_consul_tags']
      regex: '(.*)http(.*)'
      action: keep

    scrape_interval: 5s
    metrics_path: /v1/metrics
    params:
      format: ['prometheus']
EOH
      }
      driver = "docker"
      config {
        image = "prom/prometheus:latest"
        volumes = [
          "local/prometheus.yml:/etc/prometheus/prometheus.yml"
        ]
        port_map {
          prometheus_ui = 9090
        }
      }
      resources {
        network {
          mbits = 10
          port "prometheus_ui" {}
        }
      }
      service {
        name = "prometheus"
        tags = ["urlprefix-/"]
        port = "prometheus_ui"
        check {
          name     = "prometheus_ui port alive"
          type     = "http"
          path     = "/-/healthy"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
```
Notice we are using the [template][template] stanza to create a Prometheus
configuration using [environment][env] variables. In this case, we are using the
environment variable `NOMAD_IP_prometheus_ui` in the
[consul_sd_configs][consul_sd_config]
section to ensure Prometheus can use Consul to detect and scrape targets.
This works in our example because Consul is installed alongside Nomad.
Additionally, we benefit from this configuration by avoiding the need to
hard-code IP addresses. If you did not use the repo provided in this guide to
create a Nomad cluster, be sure to point your Prometheus configuration
to a Consul server you have set up.

The [volumes][volumes] option allows us to take the configuration file we
dynamically created and place it in our Prometheus container.


### Step 5: Run the Prometheus Job

We can now register our job for Prometheus:

```shell
$ nomad job run prometheus.nomad
==> Monitoring evaluation "4e6b7127"
    Evaluation triggered by job "prometheus"
    Evaluation within deployment: "d3a651a7"
    Allocation "9725af3d" created: node "28d7f859", group "monitoring"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "4e6b7127" finished with status "complete"
```
Prometheus is now deployed. You can visit any of your client nodes at port
`9999` to visit the web interface. There is only one instance of Prometheus
running in the Nomad cluster, but you are automatically routed to it
regardless of which node you visit because fabio is deployed and running on the
cluster as well.

At the top menu bar, click on `Status` and then `Targets`. You should see all
of your Nomad nodes (servers and clients) show up as targets. Please note that
the IP addresses will be different in your cluster.

[![Prometheus Targets][prometheus-targets]][prometheus-targets]

Let's use Prometheus to query how many jobs are running in our Nomad cluster.
On the main page, type `nomad_nomad_job_summary_running` into the query
section. You can also select the query from the drop-down list.

[![Running Jobs][running-jobs]][running-jobs]

You can see that the value of our fabio job is `3` since it is using the
[system][system] scheduler type. This makes sense because we are running
three Nomad clients in our demo cluster. The value of our Prometheus job, on
the other hand, is `1` since we have only deployed one instance of it.
To see the description of other metrics, visit the [telemetry][telemetry]
section.

## Next Steps

Read the Prometheus [Alerting Overview][alerting] to learn how to incorporate
[Alertmanager][alertmanager] into your Prometheus configuration to send out
notifications. 

[alerting]: https://prometheus.io/docs/alerting/overview/
[alertmanager]: https://prometheus.io/docs/alerting/alertmanager/
[configuring prometheus]: https://prometheus.io/docs/introduction/first_steps/#configuring-prometheus
[consul_sd_config]: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#%3Cconsul_sd_config%3E
[env]: /docs/runtime/environment.html
[fabio]: https://fabiolb.net/
[fabio-lb]: http://example.com
[prometheus-targets]: /assets/images/prometheus-targets.png
[running-jobs]: /assets/images/running-jobs.png
[telemetry]: /docs/agent/telemetry.html
[telemetry stanza]: /docs/agent/configuration/telemetry.html
[template]: /docs/job-specification/template.html
[volumes]: /docs/drivers/docker.html#volumes
[prometheus]: https://prometheus.io/docs/introduction/overview/
[system]: /docs/runtime/schedulers.html#system
