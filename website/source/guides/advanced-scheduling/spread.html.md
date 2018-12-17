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

Create a file with the name `redis.nomad` and place the following content in it:

```hcl
job "redis" {
 datacenters = ["dc1", "dc2"]
 type = "service"

 spread {
   attribute = "${node.datacenter}"
   weight = 100
 }

 group "cache1" {
   count = 6

   task "redis" {
     driver = "docker"

     config {
       image = "redis:latest"
       port_map {
         db = 6379
       }
     }

     resources {
       network {
         port "db" {}
       }
     }

     service {
       name = "redis-cache"
       port = "db"
       check {
         name     = "alive"
         type     = "tcp"
         interval = "10s"
         timeout  = "2s"
       }
     }
   }
 }
}
```
Note that we used the `spread` stanza and specified the [datacenter][attributes]
attribute without using any targets with the percent option. This will tell the Nomad scheduler to evenly distribute the workload between the two datacenters we have configured.

### Step 3: Register the Job `redis.nomad`

Run the Nomad job with the following command:

```shell
$ nomad run redis.nomad
==> Monitoring evaluation "c88e6a0d"
    Evaluation triggered by job "redis"
    Allocation "7b953898" created: node "622dfefb", group "cache1"
    Allocation "a46017cf" created: node "18de1c0c", group "cache1"
    Allocation "d7b8ac3a" created: node "abd5b2a8", group "cache1"
    Allocation "34477553" created: node "622dfefb", group "cache1"
    Allocation "6a3766d2" created: node "622dfefb", group "cache1"
    Allocation "788de07b" created: node "abd5b2a8", group "cache1"
    Evaluation status changed: "pending" -> "complete"
```

Note that three of the six allocations have been placed on node `622dfefb`. This
is the node we configured to be in datacenter `dc2`. The Nomad scheduler has
distributed the workload evenly between the two datacenters because of the
spread we specified.

Keep in mind that the Nomad scheduler still factors in other components into the overall scoring of nodes when making placements, so you should not expect the spread stanza to strictly implement your distribution preferences like a [constraint][constraint-stanza]. We will take a detailed look at the scoring in the next few steps.


[attributes]: /docs/runtime/interpolation.html#node-variables-
[client-metadata]: /docs/configuration/client.html#meta
[constraint-stanza]: /docs/job-specification/constraint.html
[job-specification]: /docs/job-specification/index.html
[node-status]: /docs/commands/node/status.html
[percent]: /spread-docs-coming-soon
[scheduling]: /docs/internals/scheduling.html
[spread-stanza]: /spread-docs-coming-soon
[target]: /spread-docs-coming-soon
