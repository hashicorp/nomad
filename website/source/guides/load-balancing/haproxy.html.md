---
layout: "guides"
page_title: "Load Balancing with HAProxy"
sidebar_current: "guides-load-balancing-haproxy"
description: |-
  There are multiple approaches to load balancing within a Nomad cluster.
  One approach involves using [HAProxy][haproxy] which natively integrates with
  service discovery data from Consul. 
---

# Load Balancing with HAProxy

The main use case for HAProxy in this scenario is to distribute incoming HTTP(S)
and TCP requests from the internet to frontend services that can handle these
requests. This guide will show you one such example using a demo web
application.

HAProxy version 1.8+ (LTS) includes the [server-template] directive, which lets
users specify placeholder backend servers to populate HAProxy’s load balancing
pools. Server-template can use Consul as one of these backend servers,
requesting SRV records from Consul DNS.

## Reference Material

- [HAProxy][haproxy]
- [Load Balancing Strategies for Consul][lb-strategies]

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

Note that this job deploys 3 instances of our demo web application which we will
load balance with HAProxy in the next few steps.

### Step 2: Deploy the Demo Web App

We can now deploy our demo web application:

```shell
$ nomad run webapp.nomad 
==> Monitoring evaluation "8f3af425"
    Evaluation triggered by job "demo-webapp"
    Evaluation within deployment: "dc4c1925"
    Allocation "bf9f850f" created: node "d16a11fb", group "demo"
    Allocation "25e0496a" created: node "b78e27be", group "demo"
    Allocation "a97e7d39" created: node "01d3eb32", group "demo"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "8f3af425" finished with status "complete"
```

### Step 3: Create a Job for HAProxy

Create a job for HAProxy and name it `haproxy.nomad`. This will be our load
balancer that will balance requests to the deployed instances of our web
application.

```hcl
job "haproxy" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "service"

  group "haproxy" {
    count = 1

    task "haproxy" {
      driver = "docker"

      config {
        image        = "haproxy:2.0"
        network_mode = "host"

        volumes = [
          "local/haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg",
        ]
      }

      template {
        data = <<EOF
defaults
   mode http

frontend stats
   bind *:1936
   stats uri /
   stats show-legends
   no log

frontend http_front
   bind *:8080
   default_backend http_back

backend http_back
    balance roundrobin
    server-template mywebapp 10 _demo-webapp._tcp.service.consul resolvers consul resolve-opts allow-dup-ip resolve-prefer ipv4 check

resolvers consul
  nameserver consul 127.0.0.1:53
  accepted_payload_size 8192
  hold valid 5s
EOF

        destination = "local/haproxy.cfg"
      }

      service {
        name = "haproxy"
        check {
          name     = "alive"
          type     = "tcp"
          port     = "http"
          interval = "10s"
          timeout  = "2s"
        }
      }

      resources {
        cpu    = 200
        memory = 128

        network {
          mbits = 10

          port "http" {
            static = 8080
          }

          port "haproxy_ui" {
            static = 1936
          }
        }
      }
    }
  }
}
```

Take note of the following key points from the HAProxy configuration we have
defined:

- The `balance type` under the `backend http_back` stanza in the HAProxy config
  is round robin and will load balance across the available service in order.
- The `server-template` option allows Consul service registrations to configure
  HAProxy's backend server pool. Because of this, you do not need to explicitly
  add your backend servers' IP addresses. We have specified a server-template
  named mywebapp. The template name is not tied to the service name which is
  registered in Consul.
- `_demo-webapp._tcp.service.consul` allows HAProxy to use the DNS SRV record for
  the backend service `demo-webapp.service.consul` to discover the available
  instances of the service.

Additionally, keep in mind the following points from the Nomad job spec:

- We have statically set the port of our load balancer to `8080`. This will
  allow us to query `haproxy.service.consul:8080` from anywhere inside our cluster
  so we can reach our web application.
- Please note that although we have defined the template [inline][inline], we
  could alternatively use the template stanza [in conjunction with the artifact
  stanza][remote-template] to download an input template from a remote source
  such as an S3 bucket.

### Step 4: Run the HAProxy Job

We can now register our HAProxy job:

```shell
$ nomad run haproxy.nomad 
==> Monitoring evaluation "937b1a2d"
    Evaluation triggered by job "haproxy"
    Evaluation within deployment: "e8214434"
    Allocation "53145b8b" created: node "d16a11fb", group "haproxy"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "937b1a2d" finished with status "complete"
```

### Step 5: Check the HAProxy Statistics Page

You can visit the statistics and monitoring page for HAProxy at
`http://<Your-HAProxy-IP-address>:1936`. You can use this page to verify your
settings and for basic monitoring.

[![Home Page][haproxy_ui]][haproxy_ui]

Notice there are 10 pre-provisioned load balancer backend slots for your service
but that only three of them are being used, corresponding to the three allocations in the current job.

### Step 6: Make a Request to the Load Balancer

If you query the HAProxy load balancer, you should be able to see a response
similar to the one shown below (this command should be run from a
node inside your cluster):

```shell
$ curl haproxy.service.consul:8080
Welcome! You are on node 172.31.54.242:20124
```

Note that your request has been forwarded to one of the several deployed
instances of the demo web application (which is spread across 3 Nomad clients).
The output shows the IP address of the host it is deployed on. If you repeat
your requests, you will see that the IP address changes.

* Note: if you would like to access HAProxy from outside your cluster, you
  can set up a load balancer in your environment that maps to an active port
  `8080` on your clients (or whichever port you have configured for HAProxy to
  listen on). You can then send your requests directly to your external load
  balancer.

[consul-template]: https://github.com/hashicorp/consul-template#consul-template
[consul-temp-syntax]: https://github.com/hashicorp/consul-template#service
[haproxy]: http://www.haproxy.org/
[haproxy_ui]: /assets/images/haproxy_ui.png
[inline]: /docs/job-specification/template.html#inline-template
[lb-strategies]: https://www.hashicorp.com/blog/configuring-third-party-loadbalancers-with-consul-nginx-haproxy-f5/
[reference-arch]: /guides/install/production/reference-architecture.html#high-availability
[remote-template]: /docs/job-specification/template.html#remote-template
[server-template]: https://www.haproxy.com/blog/whats-new-haproxy-1-8/#server-template-configuration-directive
[template-stanza]: /docs/job-specification/template.html
[terraform-repo]: https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud

