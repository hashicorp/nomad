---
layout: "guides"
page_title: "Load Balancing with Traefik"
sidebar_current: "guides-load-balancing-traefik"
description: |-
  There are multiple approaches to load balancing within a Nomad cluster.
  One approach involves using [Traefik][traefik] which natively integrates
  with service discovery data from Consul. 
---

# Load Balancing with Traefik 

The main use case for Traefik in this scenario is to distribute incoming HTTP(S)
and TCP requests from the internet to frontend services that can handle these
requests. This guide will show you one such example using a demo web
application.

Traefik can natively integrate with Consul using the [Consul Catalog
Provider][traefik-consul-provider] and can use [tags][traefik-tags] to route
traffic.

## Reference Material

- [Traefik][traefik]
- [Traefik Consul Catalog Provider Documentation][traefik-consul-provider] 

## Estimated Time to Complete

20 minutes

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this [repo][terraform-repo] to
easily provision a sandbox environment. This guide will assume a cluster with
one server node and three client nodes.

-> **Note:** This guide is for demo purposes and only assumes a single server
node. Please consult the [reference architecture][reference-arch] for production
configuration.

## Steps

### Step 1: Create a Job for Demo Web App

Create a job for a demo web application and name the file `webapp.nomad`:

```hcl
job "demo-webapp" {
  datacenters = ["dc1"]

  group "demo" {
    count = 3

    task "server" {
      env {
        PORT    = "${NOMAD_PORT_http}"
        NODE_IP = "${NOMAD_IP_http}"
      }

      driver = "docker"

      config {
        image = "hashicorp/demo-webapp-lb-guide"
      }

      resources {
        network {
          mbits = 10
          port  "http" {}
        }
      }

      service {
        name = "demo-webapp"
        port = "http"
        tags = [
          "traefik.tags=service",
          "traefik.frontend.rule=PathPrefixStrip:/myapp",
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

- Note that this job deploys 3 instances of our demo web application which we
  will load balance with Traefik in the next few steps.
- We are using tags to configure routing to our web app. Even though our
  application listens on `/`, it is possible to define `/myapp` as the route
  because of the [`PathPrefixStrip`][matchers] option.

### Step 2: Deploy the Demo Web App

We can now deploy our demo web application:

```shell
$ nomad run webapp.nomad 
==> Monitoring evaluation "a2061ab7"
    Evaluation triggered by job "demo-webapp"
    Evaluation within deployment: "8ca6d358"
    Allocation "1d14babe" created: node "2d6eea6e", group "demo"
    Allocation "3abb950d" created: node "a62fa99d", group "demo"
    Allocation "c65e14bf" created: node "a209a662", group "demo"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "a2061ab7" finished with status "complete"
```

### Step 3: Create a Job for Traefik

Create a job for Traefik and name it `traefik.nomad`. This will be our load
balancer that will balance requests to the deployed instances of our web
application.

```hcl
job "traefik" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "service"

  group "traefik" {
    count = 1

    task "traefik" {
      driver = "docker"

      config {
        image        = "traefik:1.7"
        network_mode = "host"

        volumes = [
          "local/traefik.toml:/etc/traefik/traefik.toml",
        ]
      }

      template {
        data = <<EOF
[entryPoints]
    [entryPoints.http]
    address = ":8080"
    [entryPoints.traefik]
    address = ":8081"

[api]

    dashboard = true

# Enable Consul Catalog configuration backend.
[consulCatalog]

endpoint = "127.0.0.1:8500"

domain = "consul.localhost"

prefix = "traefik"

constraints = ["tag==service"]
EOF

        destination = "local/traefik.toml"
      }

      resources {
        cpu    = 100
        memory = 128

        network {
          mbits = 10

          port "http" {
            static = 8080
          }

          port "api" {
            static = 8081
          }
        }
      }

      service {
        name = "traefik"
        check {
          name     = "alive"
          type     = "tcp"
          port     = "http"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
```

- We have statically set the port of our load balancer to `8080`. This will
  allow us to query `traefik.service.consul:8080` at the appropriate paths (as
  configured in the tags section of `webapp.nomad` from anywhere inside our
  cluster so we can reach our web application.
- The Traefik dashboard is configured at port `8081`.
- Please note that although we have defined the template [inline][inline], we
  could alternatively use the template stanza [in conjunction with the artifact
  stanza][remote-template] to download an input template from a remote source
  such as an S3 bucket.

### Step 4: Run the Traefik Job

We can now register our Traefik job:

```shell
$ nomad run traefik.nomad 
==> Monitoring evaluation "e22ce276"
    Evaluation triggered by job "traefik"
    Evaluation within deployment: "c6466497"
    Allocation "695c5632" created: node "a62fa99d", group "traefik"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "e22ce276" finished with status "complete"
```

### Step 5: Check the Traefik Dashboard

You can visit the dashboard for Traefik at
`http://<Your-Traefik-IP-address>:8081`. You can use this page to verify your
settings and for basic monitoring.

[![Home Page][traefik_ui]][traefik_ui]

### Step 6: Make a Request to the Load Balancer

If you query the Traefik load balancer, you should be able to see a response
similar to the one shown below (this command should be run from a
node inside your cluster):

```shell
$ curl http://traefik.service.consul:8080/myapp
Welcome! You are on node 172.31.28.103:28893
```

Note that your request has been forwarded to one of the several deployed
instances of the demo web application (which is spread across 3 Nomad clients).
The output shows the IP address of the host it is deployed on. If you repeat
your requests, you will see that the IP address changes.

* Note: if you would like to access Traefik from outside your cluster, you
  can set up a load balancer in your environment that maps to an active port
  `8080` on your clients (or whichever port you have configured for Traefik to
  listen on). You can then send your requests directly to your external load
  balancer.

[inline]: /docs/job-specification/template.html#inline-template
[matchers]: https://docs.traefik.io/v1.4/basics/#matchers
[reference-arch]: /guides/install/production/reference-architecture.html#high-availability
[remote-template]: /docs/job-specification/template.html#remote-template
[template-stanza]: /docs/job-specification/template.html
[terraform-repo]: https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud
[traefik]: https://traefik.io/
[traefik_ui]: /assets/images/traefik_ui.png
[traefik-consul-provider]: https://docs.traefik.io/v1.7/configuration/backends/consulcatalog/
[traefik-tags]: https://docs.traefik.io/v1.5/configuration/backends/consulcatalog/#tags
