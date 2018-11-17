---
layout: "guides"
page_title: "Using Prometheus to Monitor Nomad Metrics"
sidebar_current: "guides-operations-monitoring-prometheus"
description: |-
  It is possible to collect metrics on Nomad with Prometheus after enabling
  telemetry on Nomad servers and clients.
---

# Using Prometheus to Monitor Nomad Metrics

This guide explains how to configure [Prometheus][prometheus] to integrate with
a Nomad cluster and Prometheus [Alertmanager][alertmanager]. While this guide introduces the basics of enabling [telemetry][telemetry] and alerting, a Nomad operator can go much further by customizing dashboards and integrating different
[receivers][receivers] for alerts.

## Reference Material

- [Configuring Prometheus][configuring prometheus]
- [Telemetry Stanza in Nomad Agent Configuration][telemetry stanza]
- [Alerting Overview][alerting]

## Estimated Time to Complete

25 minutes

## Challenge

Think of a scenario where a Nomad operator needs to deploy Prometheus to
collect metrics from a Nomad cluster. The operator must enable telemetry on
the Nomad servers and clients as well as configure Prometheus to use Consul
for service discovery. The operator must also configure Prometheus Alertmanager
so notifications can be sent out to a specified [receiver][receivers].


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

-> **Please Note:** This guide is for demo purposes and is only using a single
server node. In a production cluster, 3 or 5 server nodes are recommended. The
alerting rules defined in this guide are for instructional purposes. Please
refer to [Alerting Rules][alertingrules] for more information.

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
        image = "fabiolb/fabio"
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

### Step 6: Deploy Alertmanager

Now that we have enabled Prometheus to collect metrics from our cluster and see
the state of our jobs, let's deploy [Alertmanager][alertmanager]. Keep in mind
that Prometheus sends alerts to Alertmanager. It is then Alertmanager's job to
send out the notifications on those alerts to any designated [receiver][receivers].

Create a job for Alertmanager and named it `alertmanager.nomad`

```hcl
job "alertmanager" {
  datacenters = ["dc1"]
  type = "service"

  group "alerting" {
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

    task "alertmanager" {
      driver = "docker"
      config {
        image = "prom/alertmanager:latest"
        port_map {
          alertmanager_ui = 9093
        }
      }
      resources {
        network {
          mbits = 10
          port "alertmanager_ui" {}
        }
      }
      service {
        name = "alertmanager"
        tags = ["urlprefix-/alertmanager strip=/alertmanager"]
        port = "alertmanager_ui"
        check {
          name     = "alertmanager_ui port alive"
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

### Step 7: Configure Prometheus to Integrate with Alertmanager

Now that we have deployed Alertmanager, let's slightly modify our Prometheus job
configuration to allow it to recognize and send alerts to it. Note that there are
some rules in the configuration that refer a to a web server we will deploy soon.

Below is the same Prometheus configuration we detailed above, but we have added
some sections that hook Prometheus into the Alertmanager and set up some Alerting
rules.

```hcl
job "prometheus" {
  datacenters = ["dc1"]
  type = "service"

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
        destination = "local/webserver_alert.yml"
        data = <<EOH
---
groups:
- name: prometheus_alerts
  rules:
  - alert: Webserver down
    expr: absent(up{job="webserver"})
    for: 10s
    labels:
      severity: critical
    annotations:
      description: "Our webserver is down."
EOH
      }

      template {
        change_mode = "noop"
        destination = "local/prometheus.yml"
        data = <<EOH
---
global:
  scrape_interval:     5s
  evaluation_interval: 5s

alerting:
  alertmanagers:
  - consul_sd_configs:
    - server: '{{ env "NOMAD_IP_prometheus_ui" }}:8500'
      services: ['alertmanager']

rule_files:
  - "webserver_alert.yml"

scrape_configs:

  - job_name: 'alertmanager'

    consul_sd_configs:
    - server: '{{ env "NOMAD_IP_prometheus_ui" }}:8500'
      services: ['alertmanager']

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

  - job_name: 'webserver'

    consul_sd_configs:
    - server: '{{ env "NOMAD_IP_prometheus_ui" }}:8500'
      services: ['webserver']

    metrics_path: /metrics
EOH
      }
      driver = "docker"
      config {
        image = "prom/prometheus:latest"
        volumes = [
          "local/webserver_alert.yml:/etc/prometheus/webserver_alert.yml",
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
Notice we have added a few important sections to this job file:

  - We added another template stanza that defines an [alerting rule][alertingrules]
    for our web server. Namely, Prometheus will send out an alert if it detects
    the `webserver` service has disappeared.

  - We added an `alerting` block to our Prometheus configuration as well as a
    `rule_files` block to make Prometheus aware of Alertmanager as well as the
    rule we have defined.

  - We are now also scraping Alertmanager along with our
    web server.

### Step 8: Deploy Web Server

Create a job for our web server and name it `webserver.nomad`

```hcl
job "webserver" {
  datacenters = ["dc1"]

  group "webserver" {
    task "server" {
      driver = "docker"
      config {
        image = "hashicorp/demo-prometheus-instrumentation:latest"
      }

      resources {
        cpu = 500
        memory = 256
        network {
          mbits = 10
          port  "http"{}
        }
      }

      service {
        name = "webserver"
        port = "http"

        tags = [
          "testweb",
          "urlprefix-/webserver strip=/webserver",
        ]

        check {
          type     = "http"
          path     = "/"
          interval = "2s"
          timeout  = "2s"
        }
      }
    }
  }
}
```
At this point, re-run your Prometheus job. After a few seconds, you will see the
web server and Alertmanager appear in your list of targets.

[![New Targets][new-targets]][new-targets]

You should also be able to go to the `Alerts` section of the Prometheus web interface
and see the alert that we have configured. No alerts are active because our web server
is up and running.

[![Alerts][alerts]][alerts]

### Step 9: Stop the Web Server

Run `nomad stop webserver` to stop our webserver. After a few seconds, you will see
that we have an active alert in the `Alerts` section of the web interface.

[![Active Alerts][active-alerts]][active-alerts]

We can now go to the Alertmanager web interface to see that Alertmanager has received
this alert as well. Since Alertmanager has been configured behind fabio, go to the IP address of any of your client nodes at port `9999` and use `/alertmanager` as the route. An example is shown below:

-> < client node IP >:9999/alertmanager

You should see that Alertmanager has received the alert.

[![Alertmanager Web UI][alertmanager-webui]][alertmanager-webui]

## Next Steps

Read more about Prometheus [Alertmanager][alertmanager] and how to configure it
to send out notifications to a [receiver][receivers] of your choice.

[active-alerts]: /assets/images/active-alert.png
[alerts]: /assets/images/alerts.png
[alerting]: https://prometheus.io/docs/alerting/overview/
[alertingrules]: https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/
[alertmanager]: https://prometheus.io/docs/alerting/alertmanager/
[alertmanager-webui]: /assets/images/alertmanager-webui.png
[configuring prometheus]: https://prometheus.io/docs/introduction/first_steps/#configuring-prometheus
[consul_sd_config]: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#%3Cconsul_sd_config%3E
[env]: /docs/runtime/environment.html
[fabio]: https://fabiolb.net/
[fabio-lb]: /guides/load-balancing/fabio.html
[new-targets]: /assets/images/new-targets.png
[prometheus-targets]: /assets/images/prometheus-targets.png
[running-jobs]: /assets/images/running-jobs.png
[telemetry]: /docs/configuration/telemetry.html
[telemetry stanza]: /docs/configuration/telemetry.html
[template]: /docs/job-specification/template.html
[volumes]: /docs/drivers/docker.html#volumes
[prometheus]: https://prometheus.io/docs/introduction/overview/
[receivers]: https://prometheus.io/docs/alerting/configuration/#%3Creceiver%3E
[system]: /docs/schedulers.html#system
