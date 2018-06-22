---
layout: "guides"
page_title: "Hardware Requirements"
sidebar_current: "guides-operations-requirements"
description: |-
  Learn about Nomad client and server requirements such as memory and CPU
  recommendations, network topologies, and more.
---

# Hardware Requirements

## Resources (RAM, CPU, etc.)

**Nomad servers** may need to be run on large machine instances. We suggest
having between 4-8+ cores, 16-32 GB+ of memory, 40-80 GB+ of **fast** disk and
significant network bandwidth. The core count and network recommendations are to
ensure high throughput as Nomad heavily relies on network communication and as
the Servers are managing all the nodes in the region and performing scheduling.
The memory and disk requirements are due to the fact that Nomad stores all state
in memory and will store two snapshots of this data onto disk, which causes high IO in busy clusters with lots of writes. Thus disk should
be at least 2 times the memory available to the server when deploying a high
load cluster. When running on AWS prefer NVME or Provisioned IOPS SSD storage for data dir.

These recommendations are guidelines and operators should always monitor the
resource usage of Nomad to determine if the machines are under or over-sized.

**Nomad clients** support reserving resources on the node that should not be
used by Nomad. This should be used to target a specific resource utilization per
node and to reserve resources for applications running outside of Nomad's
supervision such as Consul and the operating system itself.

Please see the [reservation configuration](/docs/configuration/client.html#reserved) for
more detail.

## Network Topology

**Nomad servers** are expected to have sub 10 millisecond network latencies
between each other to ensure liveness and high throughput scheduling. Nomad
servers can be spread across multiple datacenters if they have low latency
connections between them to achieve high availability.

For example, on AWS every region comprises of multiple zones which have very low
latency links between them, so every zone can be modeled as a Nomad datacenter
and every Zone can have a single Nomad server which could be connected to form a
quorum and a region.

Nomad servers uses Raft for state replication and Raft being highly consistent
needs a quorum of servers to function, therefore we recommend running an odd
number of Nomad servers in a region.  Usually running 3-5 servers in a region is
recommended. The cluster can withstand a failure of one server in a cluster of
three servers and two failures in a cluster of five servers. Adding more servers
to the quorum adds more time to replicate state and hence throughput decreases
so we don't recommend having more than seven servers in a region.

**Nomad clients** do not have the same latency requirements as servers since they
are not participating in Raft. Thus clients can have 100+ millisecond latency to
their servers. This allows having a set of Nomad servers that service clients
that can be spread geographically over a continent or even the world in the case
of having a single "global" region and many datacenter.

## Ports Used

Nomad requires 3 different ports to work properly on servers and 2 on clients,
some on TCP, UDP, or both protocols. Below we document the requirements for each
port.

* HTTP API (Default 4646). This is used by clients and servers to serve the HTTP
  API. TCP only.

* RPC (Default 4647). This is used by servers and clients to communicate among
  each other. TCP only.

* Serf WAN (Default 4648). This is used by servers to gossip both over the LAN and
  WAN to other servers. It isn't required that Nomad clients can reach this address.
  TCP and UDP.

When tasks ask for dynamic ports, they are allocated out of the port range
between 20,000 and 32,000. This is well under the ephemeral port range suggested
by the [IANA](https://en.wikipedia.org/wiki/Ephemeral_port). If your operating
system's default ephemeral port range overlaps with Nomad's dynamic port range,
you should tune the OS to avoid this overlap.

On Linux this can be checked and set as follows:

```
$ cat /proc/sys/net/ipv4/ip_local_port_range 
32768   60999
$ echo "49152 65535" > /proc/sys/net/ipv4/ip_local_port_range 
```
