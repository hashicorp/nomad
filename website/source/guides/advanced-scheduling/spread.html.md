---
layout: "guides"
page_title: "Spread"
sidebar_current: "guides-advanced-scheduling"
description: |-
  The following guide walks the user through using the spread stanza in Nomad.
---

# Increasing Failure Tolerance with Spread

The Nomad scheduler uses a bin packing algorithm when making job placements on nodes to optimize resource utilization and density of applications. Although bin packing ensures optimal resource utilization, it can lead to some nodes carrying a majority of allocations for a given job. This can cause cascading failures where the failure of a single node or a single data center can lead to application unavailability.

The [spread stanza][spread-stanza] solves this problem by allowing operators to distribute their workloads in a customized way based on [attributes][attributes] and/or [client metadata][client-metadata]. By using spread criteria in their job specification, Nomad job operators can ensure that failures across a domain such as datacenter or rack don't affect application availability.

## Reference Material

- The [spread][spread-stanza] stanza documentation
- [Scheduling][scheduling] with Nomad

## Estimated Time to Complete

20 minutes

## Challenge

Think of a scenario where a Nomad operator needs to increase the fault tolerance
of a job that is deployed across different datacenters (we will be using `dc1` and `dc2` in our example). We want to make sure that the workload is not too heavily distributed in either datacenter in case one of them goes down.

## Solution

Use the `spread` stanza in the Nomad [job specification][job-specification] to ensure the workload is being appropriately distributed between datacenters. The Nomad operator can use the `percent` option with a `target` to customize the spread.


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
1da3ff62  dc1  ip-172-31-25-111  <none>  false  eligible     ready
d1bbffa2  dc1  ip-172-31-21-89   <none>  false  eligible     ready
98f4562c  dc2  ip-172-31-27-204  <none>  false  eligible     ready
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
   target "dc1" {
     percent = 80
   }
   target "dc2" {
     percent = 20
   }
 }

 group "cache1" {
   count = 10

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
attribute while targeting `dc1` and `dc2` with the percent options. This will tell the Nomad scheduler to make an attempt to distribute 80% of the workload on `dc1` and 20% of the workload on `dc2`.

### Step 3: Register the Job `redis.nomad`

Run the Nomad job with the following command:

```shell
$ nomad run redis.nomad
==> Monitoring evaluation "54af824f"
    Evaluation triggered by job "redis"
    Allocation "0771e2fe" created: node "98f4562c", group "cache1"
    Allocation "14d1cf3b" created: node "98f4562c", group "cache1"
    Allocation "4a7fd9c3" created: node "d1bbffa2", group "cache1"
    Allocation "61fb2327" created: node "d1bbffa2", group "cache1"
    Allocation "ce6af8da" created: node "1da3ff62", group "cache1"
    Allocation "1347cf2b" created: node "1da3ff62", group "cache1"
    Allocation "3faf4c77" created: node "1da3ff62", group "cache1"
    Allocation "5b8fd14f" created: node "d1bbffa2", group "cache1"
    Allocation "8345ac3c" created: node "1da3ff62", group "cache1"
    Allocation "9c040376" created: node "d1bbffa2", group "cache1"
    Evaluation status changed: "pending" -> "complete"
```

Note that two of the ten allocations have been placed on node `98f4562c`. This is the node we configured to be in datacenter `dc2`. The Nomad scheduler has distributed 20% of the workload to `dc2` as we specified in the `spread` stanza.

Keep in mind that the Nomad scheduler still factors in other components into the overall scoring of nodes when making placements, so you should not expect the spread stanza to strictly implement your distribution preferences like a [constraint][constraint-stanza]. We will take a detailed look at the scoring in the next few steps.

### Step 4: Check the Status of the `redis` Job

At this point, we are going to check the status of our job and verify where our allocations have been placed. Run the following command:

```shell
$ nomad status redis
```

You should see 10 instances of your job running in the `Summary` section of the output as show below:

```shell
...
Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
cache1      0       0         10       0       0         0

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created    Modified
0771e2fe  98f4562c  cache1      0        run      running  3m41s ago  3m26s ago
1347cf2b  1da3ff62  cache1      0        run      running  3m41s ago  3m16s ago
14d1cf3b  98f4562c  cache1      0        run      running  3m41s ago  3m16s ago
3faf4c77  1da3ff62  cache1      0        run      running  3m41s ago  3m25s ago
4a7fd9c3  d1bbffa2  cache1      0        run      running  3m41s ago  3m18s ago
5b8fd14f  d1bbffa2  cache1      0        run      running  3m41s ago  3m19s ago
61fb2327  d1bbffa2  cache1      0        run      running  3m41s ago  3m16s ago
8345ac3c  1da3ff62  cache1      0        run      running  3m41s ago  3m26s ago
9c040376  d1bbffa2  cache1      0        run      running  3m41s ago  3m23s ago
ce6af8da  1da3ff62  cache1      0        run      running  3m41s ago  3m23s ago
```

As stated earlier, you can cross-check this output with the results of the
`nomad node status` command to verify that 20% of your workload has
been placed on the node in `dc2` (in our case, that node is `98f4562c`).

### Step 5: Obtain Detailed Scoring Information on Job Placement

As stated earlier, the Nomad scheduler will not necessarily spread your
workload in the way you have specified in the `spread` stanza even if the
resources are available. This is because spread scoring is factored in with
other metrics as well before making a scheduling decision. In this step, we will take a look at some of those other factors.

Using the output from the previous step, take any allocation that has been placed on a node and use the nomad [alloc status][alloc status] command with the [verbose][verbose] option to obtain detailed scoring information on it. In this example, we will use the allocation ID `0771e2fe` (your allocation IDs will be different).

```shell
$ nomad alloc status -verbose 0771e2fe
``` 
The resulting output will show the `Placement Metrics` section at the bottom.

```shell
...
Placement Metrics
Node                                  binpack  job-anti-affinity  node-reschedule-penalty  node-affinity  allocation-spread  final score
98f4562c-c6b7-ec1e-89b3-b583ffbac94b  0.116    0                  0                        0              0.5                0.308
1da3ff62-8e37-3fef-77d6-ca81b4eb44b4  0.217    -0.2               0                        0              0.625              0.214
d1bbffa2-19c8-9f4a-9682-eb15f5e48f4e  0.217    -0.2               0                        0              0.625              0.214
```

Note that the results from the `allocation-spread`, `binpack`, `job-anti-affinity`, `node-reschedule-penalty`, and `node-affinity` columns are combined to produce the numbers listed in the `final score` column for each node. The Nomad scheduler uses the final score for each node in deciding where to make placements.

## Next Steps

Change the values of the `percent` options on your targets in the `spread` stanza and observe how the placement behavior along with the final score given to each node changes (use the `nomad alloc status` command as shown in the previous step).

[alloc status]: /docs/commands/alloc/status.htm
[attributes]: /docs/runtime/interpolation.html#node-variables-
[client-metadata]: /docs/configuration/client.html#meta
[constraint-stanza]: /docs/job-specification/constraint.html
[job-specification]: /docs/job-specification/index.html
[node-status]: /docs/commands/node/status.html
[spread-stanza]: /docs/job-specification/spread.html
[scheduling]: /docs/internals/scheduling.html
[verbose]: /docs/commands/alloc/status.html#verbose
