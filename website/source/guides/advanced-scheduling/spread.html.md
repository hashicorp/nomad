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

Think of a scenario where a Nomad operator needs to increase the fault tolerance
of a job that is deployed across different datacenters (we will be using `dc1` and `dc2` in our example). We want to make sure that the workload is not too heavily distributed in either datacenter in case one of them goes down.

## Solution

Use the [spread][spread-stanza] stanza in the Nomad [job specification][job-specification] to ensure the workload is being evenly distributed between datacenters. The Nomad operator can use the [percent][percent] option with a [target][target] to customize the spread even further.


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

We are going to customize the spread for our job placement between the datacenters our nodes are located in. Choose one of your client nodes and edit `/etc/nomad.d/nomad.hcl` to change its location to `dc2`. A snippet of an example configuration file is show below with the required change is shown below.

```shell
data_dir = "/opt/nomad/data"
bind_addr = "0.0.0.0"
datacenter = "dc2"

# Enable the client
client {
  enabled = true
...
```
After making the change on your chosen client node, restart the Nomad service

```shell
$ sudo systemctl restart nomad
```

If everything worked correctly, you should be able to run the `nomad` [node status][node-status] command and see that one of your nodes is now in datacenter `dc2`.

```shell
$ nomad node status
ID        DC   Name              Class   Drain  Eligibility  Status
622dfefb  dc2  ip-172-31-20-105  <none>  false  eligible     ready
18de1c0c  dc1  ip-172-31-21-117  <none>  false  eligible     ready
abd5b2a8  dc1  ip-172-31-16-138  <none>  false  eligible     ready
```

### Step 2: Create a Job with the `spread` Stanza

[attributes]: /docs/runtime/interpolation.html#node-variables-
[client-metadata]: /docs/configuration/client.html#meta
[job-specification]: /docs/job-specification/index.html
[node-status]: /docs/commands/node/status.html
[percent]: /spread-docs-coming-soon
[scheduling]: /docs/internals/scheduling.html
[spread-stanza]: /spread-docs-coming-soon
[target]: /spread-docs-coming-soon
