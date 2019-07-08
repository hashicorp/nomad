---
layout: "guides"
page_title: "Consul Connect"
sidebar_current: "guides-integrations-consul-connect"
description: |-
  Learn how to use Nomad with Consul Connect to enable secure service to service communication
---

# Consul Connect

~> **Note** This page describes a new feature available in a preview release of Nomad for Hashiconf EU 2019.
The set of features described here are intended to ship with Nomad 0.10.

[Consul Connect](https://www.consul.io/docs/connect/index.html) provides service-to-service connection
authorization and encryption using mutual Transport Layer Security (TLS). Applications can use sidecar proxies in a service mesh
configuration to automatically establish TLS connections for inbound and outbound connections
without being aware of Connect at all.

# Nomad with Consul Connect Integration

Nomad integrates with Consul to provide secure service-to-service communication between
Nomad jobs and task groups. In order to support Consul Connect, Nomad adds a new networking
mode for jobs that enables tasks in the same task group to share their networking stack. With
a few changes to the job specification, job authors can opt into Connect integration. When Connect
is enabled, Nomad will launch a proxy alongside the application in the job file. The proxy (Envoy)
provides secure communication with other applications in the cluster.

Nomad job specification authors can use Nomad's Consul Connect integration to implement
[service segmentation](https://www.consul.io/segmentation.html) in a
microservice architecture running in public clouds without having to directly manage
TLS certificates. This is transparent to job specification authors as security features
in Connect continue to work even as the application scales up or down or gets rescheduled by Nomad.

# Nomad Consul Connect Example

The following section walks through an example to enable secure communication
between a web application and a Redis container. The web application and the
Redis container are managed by Nomad. Nomad additionally configures Envoy
proxies to run along side these applications. The web application is configured
to connect to Redis via localhost and Redis's default port (6379). The proxy is
managed by Nomad, and handles mTLS communication to the Redis container.

## Prerequisites

### Consul

Connect integration with Nomad requires Consul 1.6 (TODO Download link). The
Consul agent can be ran in dev mode with the following command:

```sh
$ consul agent -dev 
```

### Nomad

Nomad must schedule onto a routable interface in order for the proxies to
connect to each other. The following steps show how to start a Nomad dev agent
configured for Connect.

```sh
$ go get -u github.com/hashicorp/go-sockaddr/cmd/sockaddr
$ export DEFAULT_IFACE=$(sockaddr eval 'GetAllInterfaces | sort "default" | unique "name" | attr "name"')
$ sudo nomad agent -dev -network-interface $DEFAULT_IFACE
```

Alternatively if you know the network interface Nomad should use:

```sh
$ sudo nomad agent -dev -network-interface eth0
```

## Run Redis Container

Run the following job specification using `nomad run`. This job
uses the `network` stanza in its task group with `bridge` networking mode.
This enables the container to share its network namespace with other tasks in the
same task group. The `connect` stanza enables Consul Connect functionality for this
container. Nomad will launch a proxy for this container that registers itself in Consul.

```hcl
job "redis" {
  datacenters = ["dc1"]

  group "cache" {
    network {
      mode = "bridge"
      port "db" {
        static = 6379
      }
    }

    service {
      name = "redis-cache"
      tags = ["global", "cache"]
      port = "db"

      connect {
        sidecar_service { }
      }
    }

    task "redis" {
      driver = "docker"

      config {
        image = "redis:3.2"
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
```

## Run the web application
#### TODO change the example container

Run the following job specification using `nomad run`. This container is a web application
that uses Redis. It declares Redis as its upstream service through the `sidecar_service` stanza.
It also specifies a local bind port. This is the port on which the proxy providing secure communication
to Redis listens on.

```hcl
job "api" {
  datacenters = ["dc1"]

  group "api" {
    network {
      mode = "bridge"

      port "http" {
        to     = 8080
        static = 8080
      }
    }

    service {
      name = "api"
      port = "http"

      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "redis-cache"
              local_bind_port = 6379
            }
          }
        }
      }
    }

    task "api" {
      driver = "docker"

      config {
        image = "schmichael/rediweb:0.2"
      }

      resources {
        cpu    = 200
        memory = 100
      }
    }
  }
}
```

After running this job, visit the Nomad UI to see the Envoy proxies managed by Nomad
both for the web application and its upstream Redis service. The Consul UI also shows
the registered Connect proxies.
