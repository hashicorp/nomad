---
layout: "guides"
page_title: "Load Balancing with NGINX"
sidebar_current: "guides-load-balancing-nginx"
description: |-
  There are multiple approaches to load balancing within a Nomad cluster.
  One approach involves using [NGINX][nginx]. NGINX works well with Nomad's
  template stanza to allow for dynamic updates to its load balancing
  configuration.
---

# Load Balancing with NGINX

You can use Nomad's [template stanza][template-stanza] to configure
[NGINX][nginx] so that it can dynamically update its load balancer configuration
to scale along with your services.

The main use case for NGINX in this scenario is to distribute incoming HTTP(S)
and TCP requests from the internet to frontend services that can handle these
requests. This guide will show you one such example using a demo web
application.

## Reference Material

- [NGINX][nginx]
- [Load Balancing Strategies for Consul][lb-strategies]

## Estimated Time to Complete

20 minutes

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this [repo][terraform-repo] to
easily provision a sandbox environment. This guide will assume a cluster with
one server node and three client nodes.

-> **Note:** This guide is for demo purposes and is only using a single
server node. In a production cluster, 3 or 5 server nodes are recommended.

## Steps

### Step 1: Create a Job for Demo Web App

Create a job for a demo web application and name the file 'webapp.nomad'

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
        image = "hashicorp/demo-webapp:v1"
      }

      resources {
        network {
          mbits = 10
          port  "http"{}
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
load balance with NGINX in the next few steps.

### Step 2: Deploy the Demo Web App

We can now deploy our demo web application:

```shell
$ nomad run webapp.nomad 
==> Monitoring evaluation "ea1e8528"
    Evaluation triggered by job "demo-webapp"
    Allocation "9b4bac9f" created: node "e4637e03", group "demo"
    Allocation "c386de2d" created: node "983a64df", group "demo"
    Allocation "082653f0" created: node "f5fdf017", group "demo"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "ea1e8528" finished with status "complete"
```

### Step 3: Create a Job for NGINX

Create a job for NGINX and name it `nginx.nomad`. This will be our load balancer
that will balance requests to the deployed instances of our web application.

```hcl
job "nginx" {
  datacenters = ["dc1"]

  group "nginx" {
    count = 1

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx"

        port_map {
          http = 80
        }

        volumes = [
          "local:/etc/nginx/conf.d",
        ]
      }

      template {
        data = <<EOF
upstream backend {
{{ range service "demo-webapp" }}
  server {{ .Address }}:{{ .Port }};
{{ end }}
}

server {
   listen 80;

   location / {
      proxy_pass http://backend;
   }
}
EOF

        destination = "local/load-balancer.conf"
      }

      resources {
        network {
          mbits = 10

          port "http" {
            static = 8080
          }
        }
      }

      service {
        name = "nginx"
        port = "http"
      }
    }
  }
}
```

- We are using Nomad's [template][template-stanza] to populate the load balancer
  configuration for NGINX. The underlying tool being used is [Consul
  Template][consul-template]. You can use Consul Template's documentation to
  learn more about the [syntax][consul-temp-syntax] needed to interact with
  Consul. In this case, we are going to query the address and port of our demo
  service called `demo-webapp`.
- We have statically set the port of our load balancer to `8080`. This will
  allow us to query `nginx.service.consul:8080` from anywhere inside our cluster
  so we can reach our web application.
- Please note that although we have defined the template [inline][inline], we
  can use the template stanza [in conjunction with the artifact
  stanza][remote-template] to download an input template from a remote source
  such as an S3 bucket.

### Step 4: Run the NGINX Job

We can now register our NGINX job:

```shell
$ nomad run nginx.nomad 
==> Monitoring evaluation "45da5a89"
    Evaluation triggered by job "nginx"
    Allocation "c7f8af51" created: node "983a64df", group "nginx"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "45da5a89" finished with status "complete"
```

### Step 5: Make a Request to the Load Balancer

If you query the NGINX load balancer, you should be able to see a response similar to the one shown below:

```shell
$ curl nginx.service.consul:8080
<!DOCTYPE html>
<html>
<body>

<h1 style="color:red;">Welcome! This is <i>version 1</i> of your application!</h1>
<h1 style="color:red;">You are on node 172.31.48.118</h1>

</body>
</html>
```

Note that your request has been forwarded to one of the several deployed
instances of the demo web application (which is spread across 3 Nomad clients).
The output shows the IP address of the host it is deployed on. If you repeat
your requests, you will see that the IP address changes.

[consul-template]: https://github.com/hashicorp/consul-template#consul-template
[consul-temp-syntax]: https://github.com/hashicorp/consul-template#service
[inline]: /docs/job-specification/template.html#inline-template
[lb-strategies]: https://www.hashicorp.com/blog/configuring-third-party-loadbalancers-with-consul-nginx-haproxy-f5/
[nginx]: https://www.nginx.com/
[remote-template]: /docs/job-specification/template.html#remote-template
[template-stanza]: /docs/job-specification/template.html
[terraform-repo]: https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud

