---
layout: "guides"
page_title: "Automatic Clustering with Consul"
sidebar_current: "guides-operations-cluster-automatic"
description: |-
  Learn how to automatically bootstrap a Nomad cluster using Consul. By having
  a Consul agent installed on each host, Nomad can automatically discover other
  clients and servers to bootstrap the cluster without operator involvement.
---

# Automatic Clustering with Consul

To automatically bootstrap a Nomad cluster, we must leverage another HashiCorp
open source tool, [Consul](https://www.consul.io/). Bootstrapping Nomad is
easiest against an existing Consul cluster. The Nomad servers and clients
will become informed of each other's existence when the Consul agent is
installed and configured on each host. As an added benefit, integrating Consul
with Nomad provides service and health check registration for applications which
later run under Nomad.

Consul models infrastructures as datacenters and multiple Consul datacenters can
be connected over the WAN so that clients can discover nodes in other
datacenters. Since Nomad regions can encapsulate many datacenters, we recommend
running a Consul cluster in every Nomad datacenter and connecting them over the
WAN. Please refer to the Consul guide for both
[bootstrapping](https://www.consul.io/docs/guides/bootstrapping.html) a single
datacenter and [connecting multiple Consul clusters over the
WAN](https://www.consul.io/docs/guides/datacenters.html).

If a Consul agent is installed on the host prior to Nomad starting, the Nomad
agent will register with Consul and discover other nodes.

For servers, we must inform the cluster how many servers we expect to have. This
is required to form the initial quorum, since Nomad is unaware of how many peers
to expect. For example, to form a region with three Nomad servers, you would use
the following Nomad configuration file:

```hcl
# /etc/nomad.d/server.hcl

data_dir = "/etc/nomad.d"

server {
  enabled          = true
  bootstrap_expect = 3
}
```

This configuration would be saved to disk and then run:

```shell
$ nomad agent -config=/etc/nomad.d/server.hcl
```

A similar configuration is available for Nomad clients:

```hcl
# /etc/nomad.d/client.hcl

datacenter = "dc1"
data_dir   = "/etc/nomad.d"

client {
  enabled = true
}
```

The agent is started in a similar manner:

```shell
$ nomad agent -config=/etc/nomad.d/client.hcl
```

As you can see, the above configurations include no IP or DNS addresses between
the clients and servers. This is because Nomad detected the existence of Consul
and utilized service discovery to form the cluster.

## Internals

~> This section discusses the internals of the Consul and Nomad integration at a
very high level. Reading is only recommended for those curious to the
implementation.

As discussed in the previous section, Nomad merges multiple configuration files
together, so the `-config` may be specified more than once:

```shell
$ nomad agent -config=base.hcl -config=server.hcl
```

In addition to merging configuration on the command line, Nomad also maintains
its own internal configurations (called "default configs") which include sane
base defaults. One of those default configurations includes a "consul" block,
which specifies sane defaults for connecting to and integrating with Consul. In
essence, this configuration file resembles the following:

```hcl
# You do not need to add this to your configuration file. This is an example
# that is part of Nomad's internal default configuration for Consul integration.
consul {
  # The address to the Consul agent.
  address = "127.0.0.1:8500"

  # The service name to register the server and client with Consul.
  server_service_name = "nomad"
  client_service_name = "nomad-client"

  # Enables automatically registering the services.
  auto_advertise = true

  # Enabling the server and client to bootstrap using Consul.
  server_auto_join = true
  client_auto_join = true
}
```

Please refer to the [Consul
documentation](/docs/configuration/consul.html) for the complete set of
configuration options.
