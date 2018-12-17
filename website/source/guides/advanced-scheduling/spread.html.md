---
layout: "guides"
page_title: "Spread"
sidebar_current: "guides-advanced-scheduling"
description: |-
    The spread stanza allows operators to increase failure tolerance in their Nomad clusters by allowing them to distribute their workloads in a customized way based on attributes or client metadata.
---

# Increasing Failure Tolerance with Spread

The Nomad scheduler uses a bin packing algorithm when making job placements on
nodes to optimize resource utilization and density of applications. Although
this feature ensures that cluster resources are being used efficiently, it does
not necessarily promote maximum failure tolerance of jobs across nodes.

The [spread stanza][spread-stanza] solves this problem by allowing operators to distribute their workloads in a customized way based on [attributes][attributes] and/or [client metadata][client-metadata]. By implementing spread, Nomad operators can ensure maximum levels of failure tolerance based on their specific architectures.

## Reference Material

- The [spread][spread-stanza] stanza documentation
- [Scheduling][scheduling] with Nomad

## Estimated Time to Complete

20 minutes

## Challenge


## Solution


## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this
[repo](https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud)
to easily provision a sandbox environment. This guide will assume a cluster with
one server node and three client nodes.

-> **Please Note:** This guide is for demo purposes and is only using a single
server
node. In a production cluster, 3 or 5 server nodes are recommended.

## Steps

### Step 1: Place One of the Client Nodes in a Different Datacenter

[attributes]: /docs/runtime/interpolation.html#node-variables-
[client-metadata]: /docs/configuration/client.html#meta
[scheduling]: /docs/internals/scheduling.html
[spread-stanza]: /spread-docs-coming-soon
