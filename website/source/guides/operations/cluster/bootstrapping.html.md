---
layout: "guides"
page_title: "Clustering"
sidebar_current: "guides-operations-cluster"
description: |-
  Learn how to cluster Nomad.
---

# Clustering

Nomad models infrastructure into regions and datacenters. Servers reside at the
regional layer and manage all state and scheduling decisions for that region.
Regions contain multiple datacenters, and clients are registered to a single
datacenter (and thus a region that contains that datacenter). For more details on
the architecture of Nomad and how it models infrastructure see the [architecture
page](/docs/internals/architecture.html).

There are multiple strategies available for creating a multi-node Nomad cluster:

1. <a href="/guides/operations/cluster/manual.html">Manual Clustering</a>
1. <a href="/guides/operations/cluster/automatic.html">Automatic Clustering with Consul</a>
1. <a href="/guides/operations/cluster/cloud_auto_join.html">Cloud Auto-join</a>


Please refer to the specific documentation links above or in the sidebar for
more detailed information about each strategy.
