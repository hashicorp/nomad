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

Consider a Nomad application that needs to be deployed to multiple datacenters within a region. Datacenter `dc1` has four nodes while `dc2` has one node. This application has 10 instances and 7 of them must be deployed to `dc1` since it receives more user traffic and we need to make sure the application doesn't suffer downtime due to not enough running instances to process requests. The remaining 3 allocations can be deployed to `dc2`.

## Solution

Use the `spread` stanza in the Nomad [job specification][job-specification] to ensure the 70% of the workload is being placed in datacenter `dc1` and 30% is being placed in `dc2`. The Nomad operator can use the [percent][percent] option with a [target][target] to customize the spread.

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this [repo](https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud) to easily provision a sandbox environment. This guide will assume a cluster with one server node and five client nodes.

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
5d16d949  dc2  ip-172-31-62-240  <none>  false  eligible     ready
7b381152  dc1  ip-172-31-59-115  <none>  false  eligible     ready
10cc48cc  dc1  ip-172-31-58-46   <none>  false  eligible     ready
93f1e628  dc1  ip-172-31-58-113  <none>  false  eligible     ready
12894b80  dc1  ip-172-31-62-90   <none>  false  eligible     ready
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
     percent = 70
   }
   target "dc2" {
     percent = 30
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
attribute while targeting `dc1` and `dc2` with the percent options. This will tell the Nomad scheduler to make an attempt to distribute 70% of the workload on `dc1` and 30% of the workload on `dc2`.

### Step 3: Register the Job `redis.nomad`

Run the Nomad job with the following command:

```shell
$ nomad run redis.nomad
==> Monitoring evaluation "c3dc5ebd"
    Evaluation triggered by job "redis"
    Allocation "7a374183" created: node "5d16d949", group "cache1"
    Allocation "f4361df1" created: node "7b381152", group "cache1"
    Allocation "f7af42dc" created: node "5d16d949", group "cache1"
    Allocation "0638edf2" created: node "10cc48cc", group "cache1"
    Allocation "49bc6038" created: node "12894b80", group "cache1"
    Allocation "c7e5679a" created: node "5d16d949", group "cache1"
    Allocation "cf91bf65" created: node "7b381152", group "cache1"
    Allocation "d16b606c" created: node "12894b80", group "cache1"
    Allocation "27866df0" created: node "93f1e628", group "cache1"
    Allocation "8531a6fc" created: node "7b381152", group "cache1"
    Evaluation status changed: "pending" -> "complete"
```

Note that three of the ten allocations have been placed on node `5d16d949`. This is the node we configured to be in datacenter `dc2`. The Nomad scheduler has distributed 30% of the workload to `dc2` as we specified in the `spread` stanza.

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
0638edf2  10cc48cc  cache1      0        run      running  2m20s ago  2m ago
27866df0  93f1e628  cache1      0        run      running  2m20s ago  1m57s ago
49bc6038  12894b80  cache1      0        run      running  2m20s ago  1m58s ago
7a374183  5d16d949  cache1      0        run      running  2m20s ago  2m1s ago
8531a6fc  7b381152  cache1      0        run      running  2m20s ago  2m2s ago
c7e5679a  5d16d949  cache1      0        run      running  2m20s ago  1m55s ago
cf91bf65  7b381152  cache1      0        run      running  2m20s ago  1m57s ago
d16b606c  12894b80  cache1      0        run      running  2m20s ago  2m1s ago
f4361df1  7b381152  cache1      0        run      running  2m20s ago  2m3s ago
f7af42dc  5d16d949  cache1      0        run      running  2m20s ago  1m54s ago
```

You can cross-check this output with the results of the `nomad node status` command to verify that 30% of your workload has been placed on the node in `dc2` (in our case, that node is `5d16d949`).

### Step 5: Obtain Detailed Scoring Information on Job Placement

The Nomad scheduler will not always spread your
workload in the way you have specified in the `spread` stanza even if the
resources are available. This is because spread scoring is factored in with
other metrics as well before making a scheduling decision. In this step, we will take a look at some of those other factors.

Using the output from the previous step, take any allocation that has been placed on a node and use the nomad [alloc status][alloc status] command with the [verbose][verbose] option to obtain detailed scoring information on it. In this example, we will use the allocation ID `0638edf2` (your allocation IDs will be different).

```shell
$ nomad alloc status -verbose 0638edf2 
``` 
The resulting output will show the `Placement Metrics` section at the bottom.

```shell
...
Placement Metrics
Node                                  node-affinity  allocation-spread  binpack  job-anti-affinity  node-reschedule-penalty  final score
10cc48cc-2913-af54-74d5-d7559f373ff2  0              0.429              0.33     0                  0                        0.379
93f1e628-e509-b1ab-05b7-0944056f781d  0              0.429              0.515    -0.2               0                        0.248
12894b80-4943-4d5c-5716-c626c6b99be3  0              0.429              0.515    -0.2               0                        0.248
7b381152-3802-258b-4155-6d7dfb344dd4  0              0.429              0.515    -0.2               0                        0.248
5d16d949-85aa-3fd3-b5f4-51094cbeb77a  0              0.333              0.515    -0.2               0                        0.216
```

Note that the results from the `allocation-spread`, `binpack`, `job-anti-affinity`, `node-reschedule-penalty`, and `node-affinity` columns are combined to produce the numbers listed in the `final score` column for each node. The Nomad scheduler uses the final score for each node in deciding where to make placements.

## Next Steps

Change the values of the `percent` options on your targets in the `spread` stanza and observe how the placement behavior along with the final score given to each node changes (use the `nomad alloc status` command as shown in the previous step).

[alloc status]: /docs/commands/alloc/status.html
[attributes]: /docs/runtime/interpolation.html#node-variables-
[client-metadata]: /docs/configuration/client.html#meta
[constraint-stanza]: /docs/job-specification/constraint.html
[job-specification]: /docs/job-specification/index.html
[node-status]: /docs/commands/node/status.html
[percent]: /docs/job-specification/spread.html#percent
[spread-stanza]: /docs/job-specification/spread.html
[scheduling]: /docs/internals/scheduling/scheduling.html
[target]: /docs/job-specification/spread.html#target
[verbose]: /docs/commands/alloc/status.html#verbose
