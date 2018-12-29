---
layout: "guides"
page_title: "Load Balancing with Nomad"
sidebar_current: "guides-load-balancing"
description: |-
  There are multiple approaches to load balancing within a Nomad cluster.
  One approach involves using [fabio][fabio]. Fabio integrates natively
  with Consul and provides rich features with an optional Web UI.
---

# Load Balancing with Fabio

[Fabio][fabio] integrates natively with Consul and provides an optional Web UI
to visualize routing.

The main use case for fabio is to distribute incoming HTTP(S) and TCP requests
from the internet to frontend services that can handle these requests. This
guide will show you one such example using [Apache][apache] web server.

## Reference Material

- [Fabio](https://github.com/fabiolb/fabio) on GitHub
- [Load Balancing Strategies for Consul](https://www.hashicorp.com/blog/load-balancing-strategies-for-consul)
- [Elastic Load Balancing][elb]

## Estimated Time to Complete

20 minutes

## Challenge

Think of a scenario where a Nomad operator needs to configure an environment to
make Apache web server highly available behind an endpoint and distribute
incoming traffic evenly.

## Solution

Deploy fabio as a
[system][system]
scheduler so that it can route incoming traffic evenly to the Apache web server
group regardless of which client nodes Apache is running on. Place all client nodes
behind an [AWS load balancer][elb] to
provide the end user with a single endpoint for access.

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this
[repo](https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud)
to easily provision a sandbox environment. This guide will assume a cluster with
one server node and three client nodes.

-> **Please Note:** This guide is for demo purposes and is only using a single server
node. In a production cluster, 3 or 5 server nodes are recommended.

## Steps

### Step 1: Create a Job for Fabio

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
        cpu    = 200
        memory = 128
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

Setting `type` to [system][system] will ensure that fabio is run on all clients.
Please note that the `network_mode` option is set to `host` so that fabio can
communicate with Consul which is also running on the client nodes.

### Step 2: Run the Fabio Job

We can now register our fabio job:

```shell
$ nomad job run fabio.nomad 
==> Monitoring evaluation "fba4f04a"
    Evaluation triggered by job "fabio"
    Allocation "6e6367d4" created: node "f3739267", group "fabio"
    Allocation "d17573b4" created: node "28d7f859", group "fabio"
    Allocation "f3ad9b16" created: node "510898b6", group "fabio"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "fba4f04a" finished with status "complete"
```
At this point, you should be able to visit any one of your client nodes at port
`9998` and see the web interface for fabio. The routing table will be empty
since we have not yet deployed anything that fabio can route to.
Accordingly, if you visit any of the client nodes at port `9999` at this
point, you will get a `404` HTTP response. That will change soon.

### Step 3: Create a Job for Apache Web Server

Create a job for Apache and name it `webserver.nomad`

```hcl
job "webserver" {
  datacenters = ["dc1"]
  type = "service"

  group "webserver" {
    count = 3
    restart {
      attempts = 2
      interval = "30m"
      delay = "15s"
      mode = "fail"
    }
    ephemeral_disk {
      size = 300
    }

    task "apache" {
      driver = "docker"
      config {
        image = "httpd:latest"
        port_map {
          http = 80
        }
      }

      resources {
        network {
          mbits = 10
          port "http" {}
        }
      }

      service {
        name = "apache-webserver"
        tags = ["urlprefix-/"]
        port = "http"
        check {
          name     = "alive"
          type     = "http"
          path     = "/"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
```

Notice the tag in the service stanza begins with `urlprefix-`. This is how a
path is registered with fabio. In this case, we are registering the path '/'
with fabio (which will route us to the default page for Apache web server). 

### Step 4: Run the Job for Apache Web Server

We can now register our job for Apache:

```shell
$ nomad job run webserver.nomad 
==> Monitoring evaluation "c7bcaf40"
    Evaluation triggered by job "webserver"
    Evaluation within deployment: "e3603b50"
    Allocation "20951ad4" created: node "510898b6", group "webserver"
    Allocation "43807686" created: node "28d7f859", group "webserver"
    Allocation "7b60eb24" created: node "f3739267", group "webserver"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "c7bcaf40" finished with status "complete"
```
You have now deployed and registered your web servers with fabio! At this point,
you should be able to visit any of the Nomad clients at port `9999` and
see the default web page for Apache web server. If you visit fabio's web
interface by going to any of the client nodes at port `9998`, you will see that
the routing table has been populated as shown below (**Note:** your destination IP
addresses will be different).

[![Routing Table][routing-table]][routing-table]

Feel free to reduce the `count` in `webserver.nomad` for testing purposes. You
will see that you still get routed to the Apache home page by accessing
any client node on port `9999`. Accordingly, the routing table
in the web interface on port `9999` will reflect the changes.

### Step 5: Place Nomad Client Nodes Behing AWS Load Balancer

At this point, you are ready to place your Nomad client nodes behind an AWS load
balancer. Your Nomad client nodes may change over time, and it is important
to provide your end users with a single endpoint to access your services. This guide will use the [Classic Load Balancer][classic-lb].

The AWS [documentation][classic-lb-doc] provides instruction on how to create a
load balancer. The basic steps involve creating a load balancer, registering
instances behind the load balancer (in our case these will be the Nomad client
nodes), creating listeners, and configuring health checks.

Once you are done
with this, you should be able to hit the DNS name of your load balancer at port
80 (or whichever port you configured in your listener) and see the home page of
Apache web server. If you configured your listener to also forward traffic to
the web interface at port `9998`, you should be able to access that as well.

[![Home Page][lb-homepage]][lb-homepage]

[![Routing Table][lb-routing-table]][lb-routing-table]

[apache]: https://httpd.apache.org/
[classic-lb]: https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/introduction.html
[classic-lb-doc]: https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/elb-getting-started.html
[elb]: https://aws.amazon.com/elasticloadbalancing/
[fabio]: https://fabiolb.net/
[lb-homepage]: /assets/images/lb-homepage.png
[lb-routing-table]: /assets/images/lb-routing-table.png
[routing-table]: /assets/images/routing-table.png
[system]: /docs/schedulers.html#system
