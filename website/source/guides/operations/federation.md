---
layout: "guides"
page_title: "Multi-region Federation"
sidebar_current: "guides-operations-federation"
description: |-
  Learn how to join Nomad servers across multiple regions so users can submit
  jobs to any server in any region using global federation.
---

# Multi-region Federation

Because Nomad operates at a regional level, federation is part of Nomad core.
Federation enables users to submit jobs or interact with the HTTP API targeting
any region, from any server, even if that server resides in a different region.

Federating multiple Nomad clusters requires network connectivity between the
clusters. Servers in each cluster must be able to communicate over [RPC and
Serf][ports]. Federated clusters are expected to communicate over WANs, so they
do not need the same low latency as servers within a region.

Once Nomad servers are able to connect, federating is as simple as joining the
servers. From any server in one region, issue a join command to a server in a
remote region:

```shell
$ nomad server-join 1.2.3.4:4648
```

Note that only one join command is required per region. Servers across regions
discover other servers in the cluster via the gossip protocol and hence it's
enough to join just one known server.

If bootstrapped via Consul and the Consul clusters in the Nomad regions are
federated, then federation occurs automatically.

[ports]: /guides/operations/requirements.html#ports-used
