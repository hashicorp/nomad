---
layout: "docs"
page_title: "Bootstrapping a Nomad Cluster"
sidebar_current: "docs-cluster-bootstrap"
description: |-
  Learn how to bootstrap a Nomad cluster.
---

# Bootstrapping a Nomad Cluster

Nomad models infrastructure into regions and datacenters. Servers reside at the
regional layer and manage all state and scheduling decisions for that region.
Regions contain multiple datacenters, and clients are registered to a single
datacenter (and thus a region that contains that datacenter). For more details on
the architecture of Nomad and how it models infrastructure see the [architecture
page](/docs/internals/architecture.html).

There are two strategies for bootstrapping a Nomad cluster:

1. <a href="/docs/cluster/automatic.html">Automatic bootstrapping</a>
1. <a href="/docs/cluster/manual.html">Manual bootstrapping</a>

Please refer to the specific documentation links above or in the sidebar for
more detailed information about each strategy.
