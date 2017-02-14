---
layout: "guides"
page_title: "Federating a Nomad Cluster"
sidebar_current: "guides-cluster-federation"
description: |-
  Learn how to join Nomad servers across multiple regions so users can submit
  jobs to any server in any region using global federation.
---

# Federating a Cluster

Because Nomad operates at a regional level, federation is part of Nomad core.
Federation enables users to submit jobs or interact with the HTTP API targeting
any region, from any server, even if that server resides in a different region.

Federating multiple Nomad clusters is as simple as joining servers. From any
server in one region, issue a join command to a server in a remote region:

```shell
$ nomad server-join 1.2.3.4:4648
```

Note that only one join command is required per region. Servers across regions
discover other servers in the cluster via the gossip protocol and hence it's
enough to join just one known server.

If bootstrapped via Consul and the Consul clusters in the Nomad regions are
federated, then federation occurs automatically.
