---
layout: "docs"
page_title: "Creating a Nomad Cluster"
sidebar_current: "docs-cluster-bootstrap"
description: |-
  Learn how to bootstrap a Nomad cluster.
---

# Creating a cluster

Nomad clusters in production comprises of a few Nomad servers (an odd number,
preferably 3 or 5, but never an even number to prevent split-brain), clients and
optionally Consul servers and clients. Before we start discussing the specifics
around bootstrapping clusters we should discuss the network topology. Nomad
models infrastructure as regions and datacenters. Nomad regions may contain multiple
datacenters. Nomad servers are assigned to the same region and hence a region is a
single scheduling domain in Nomad. Each cluster of Nomad servers support
only one region, however multiple regions can be stitched together to allow a
globally coherent view of an organization's resources.


## Consul Cluster

Bootstrapping a Nomad cluster becomes significantly easier if operators use
Consul. Network topology of a Consul cluster is slightly different than Nomad.
Consul models infrastructures as datacenters and multiple Consul datacenters can
be connected over the WAN so that clients can discover nodes in other
datacenters. We recommend running a Consul Cluster in every Nomad datacenter and
connecting them over the WAN. Please refer to the Consul
[documentation](https://www.consul.io/docs/commands/join.html) to learn more
about bootstrapping Consul and connecting multiple Consul clusters over the WAN.


## Nomad Servers

Nomad servers are expected to have sub 10 millisecond network latencies between
them. Nomad servers could be spread across multiple datacenters, if they have
low latency connections between them, to achieve high availability. For example,
on AWS every region comprises of multiple zones which have very low latency
links between them, so every zone can be modeled as a Nomad datacenter and
every Zone can have a single Nomad server which could be connected to form a
quorum and form a region. Nomad servers uses Raft for replicating state between
them and Raft being highly consistent needs a quorum of servers to function,
therefore we recommend running an odd number of Nomad servers in a region.
Usually running 3-5 servers in a region is recommended. The cluster can
withstand a failure of one server in a cluster of three servers and two failures
in a cluster of five servers. Adding more servers to the quorum adds more time
to replicate state and hence throughput decreases so we don't recommend having
more than seven servers in a region.

During the bootstrapping phase Nomad servers need to know the addresses of other
servers.  Nomad will automatically bootstrap itself when the Consul service is
present, or can be manually joined by using the
[`-retry-join`](https://www.nomadproject.io/docs/agent/config.html#_retry_join)
CLI command or the Server
[`retry_join`](https://www.nomadproject.io/docs/agent/config.html#retry_join)
option.



## Nomad Clients

Nomad clients are organized in datacenters and need to be made aware of the
Nomad servers to communicate with.  If Consul is present, Nomad will
automatically self-bootstrap, otherwise they will need to be provided with a
static list of
[`servers`](https://www.nomadproject.io/docs/agent/config.html#servers) to find
the list of Nomad servers.

Operators can either place the addresses of the Nomad servers in the client
configuration or point Nomad client to the Nomad server service in Consul. Once
a client establishes connection with a Nomad servers, if new servers are added
to the cluster the addresses are propagated down to the clients along with
heartbeat.


### Bootstrapping a Nomad cluster without Consul

At least one Nomad server's address (also known as the seed node) needs to be
known ahead of time and a running agent can be joined to a cluster by running
the [`server-join` command](/docs/commands/server-join.html). 

For example, once a Nomad agent starts in the server mode it can be joined to an
existing cluster with a server whose IP is known. Once the agent joins the other
node in the cluster, it can discover the other nodes via the gossip protocol.

```
nomad server-join -retry-join 10.0.0.1
```

The `-retry-join` parameter indicates that the agent should keep trying to join
the server even if the first attempt fails. This is essential when the other
address is going to be eventually available after some time as nodes might take
a variable amount of time to boot up in a cluster.

On the client side, the addresses of the servers are expected to be specified
via the client configuration.

```
client {
    ...
    servers = ["10.10.11.2:4648", "10.10.11.3:4648", "10.10.11.4:4648"]
    ...
}
```

In the above example we are specifying three servers for the clients to
connect. If servers are added or removed, clients know about them via the
heartbeat of a server which is alive.


### Bootstrapping a Nomad cluster with Consul

Bootstrapping a Nomad cluster is significantly easier if Consul is used along
with Nomad. If a local Consul cluster is bootstrapped before Nomad, the
following configuration would register the Nomad agent with Consul and look up
the addresses of the other Nomad server addresses and join with them
automatically.

```
{
    "server_service_name": "nomad",
    "server_auto_join": true,
    "client_service_name": "nomad-client",
    "client_auto_join": true
}
```

With the above configuration Nomad agent is going to look up Consul for
addresses of agents in the `nomad` service and join them automatically.  In
addition, if the `auto-advertise` option is set Nomad is going to register the
agents with Consul automatically too. By default, Nomad will automatically
register the server and the client agents with Consul and try to auto-discover
the servers if it can talk to a local Consul agent on the same server.

Please refer to the [documentation](/docs/agent/config.html#consul_options)
for the complete set of configuration options.


### Federating a cluster

Nomad clusters across multiple regions can be federated allowing users to submit
jobs or interact with the HTTP API targeting any region, from any server.

Federating multiple Nomad clusters is as simple as joining servers. From any
server in one region, simply issue a join command to a server in the remote
region:

```
nomad server-join 10.10.11.8:4648
```

Servers across regions discover other servers in the cluster via the gossip
protocol and hence it enough to join one known server.
