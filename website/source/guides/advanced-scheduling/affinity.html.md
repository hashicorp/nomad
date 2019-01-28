---
layout: "guides"
page_title: "Affinity"
sidebar_current: "guides-advanced-scheduling"
description: |-
   The following guide walks the user through using the affinity stanza in Nomad.
---

# Expressing Job Placement Preferences with Affinities

The [affinity][affinity-stanza] stanza allows operators to express placement preferences for their jobs on particular types of nodes. Note that there is a key difference between the [constraint][constraint] stanza and the affinity stanza. The constraint stanza strictly filters where jobs are run based on [attributes][attributes] and [client metadata][client-metadata]. If no nodes are found to match, the placement does not succeed. The affinity stanza acts like a "soft constraint." Nomad will attempt to match the desired affinity, but placement will succeed even if no nodes match the desired criteria. This is done in conjunction with scoring based on the Nomad scheduler's bin packing algorithm which you can read more about [here][scheduling].

## Reference Material

- The [affinity][affinity-stanza] stanza documentation
- [Scheduling][scheduling] with Nomad

## Estimated Time to Complete

20 minutes

## Challenge

Your application can run in datacenters `dc1` and `dc2`, but you have a strong preference to run it in `dc2`. Configure your job to tell the scheduler your preference while still allowing it to place your workload in `dc1` if the desired resources aren't available.

## Solution

Specify an affinity with the proper [weight][weight] so that the Nomad scheduler can find the best nodes on which to place your job. The affinity weight will be included when scoring nodes for placement along with other factors like the bin packing algorithm.

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this
[repo](https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud)
to easily provision a sandbox environment. This guide will assume a cluster with
one server node and three client nodes.

-> **Please Note:** This guide is for demo purposes and is only using a single server
node. In a production cluster, 3 or 5 server nodes are recommended.

## Steps

### Step 1: Place One of the Client Nodes in a Different Datacenter

We are going express our job placement preference based on the datacenter our
nodes are located in. Choose one of your client nodes and edit `/etc/nomad.d/nomad.hcl` to change its location to `dc2`. A snippet of an example configuration file is show below with the required change is shown below.

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
3592943e  dc1  ip-172-31-27-159  <none>  false  eligible     ready
3dea0188  dc1  ip-172-31-16-175  <none>  false  eligible     ready
6b6e9518  dc2  ip-172-31-27-25   <none>  false  eligible     ready
```

### Step 2: Create a Job with the `affinity` Stanza

Create a file with the name `redis.nomad` and place the following content in it:

```hcl
job "redis" {
 datacenters = ["dc1", "dc2"]
 type = "service"

 affinity {
   attribute = "${node.datacenter}"
   value = "dc2"
   weight = 100
 }

 group "cache1" {
   count = 4

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
Note that we used the `affinity` stanza and specified `dc2` as the
value for the [attribute][attributes] `${node.datacenter}`. We used the value `100` for the [weight][weight] which will cause the Nomad schedular to rank nodes in datacenter `dc2` with a higher score. Keep in mind that weights can range from -100 to 100, inclusive. Negative weights serve as anti-affinities which cause Nomad to avoid placing allocations on nodes that match the criteria.

### Step 3: Register the Job `redis.nomad`

Run the Nomad job with the following command:

```shell
$ nomad run redis.nomad 
==> Monitoring evaluation "11388ef2"
    Evaluation triggered by job "redis"
    Allocation "0dfcf0ba" created: node "6b6e9518", group "cache1"
    Allocation "89a9aae9" created: node "3592943e", group "cache1"
    Allocation "9a00f742" created: node "6b6e9518", group "cache1"
    Allocation "fc0f21bc" created: node "3dea0188", group "cache1"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "11388ef2" finished with status "complete"
```

Note that two of the allocations in this example have been placed on node `6b6e9518`. This is the node we configured to be in datacenter `dc2`. The Nomad scheduler selected this node because of the affinity we specified. All of the allocations have not been placed on this node because the Nomad scheduler considers other factors in the scoring such as bin packing. This helps avoid placing too many instances of the same job on a node and prevents reduced capacity during a node level failure. We will take a detailed look at the scoring in the next few steps.

### Step 4: Check the Status of the `redis` Job

At this point, we are going to check the status of our job and verify where our
allocations have been placed. Run the following command:

```shell
$ nomad status redis
```

You should see 4 instances of your job running in the `Summary` section of the
output as show below:

```shell
...
Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
cache1      0       0         4        0       0         0

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created    Modified
0dfcf0ba  6b6e9518  cache1      0        run      running  1h44m ago  1h44m ago
89a9aae9  3592943e  cache1      0        run      running  1h44m ago  1h44m ago
9a00f742  6b6e9518  cache1      0        run      running  1h44m ago  1h44m ago
fc0f21bc  3dea0188  cache1      0        run      running  1h44m ago  1h44m ago
```

You can cross-check this output with the results of the `nomad node status` command to verify that the majority of your workload has been placed on the node in `dc2` (in our case, that node is `6b6e9518`).

### Step 5: Obtain Detailed Scoring Information on Job Placement

The Nomad scheduler will not always place all of your workload on nodes you have specified in the `affinity` stanza even if the resources are available. This is because affinity scoring is combined with other metrics as well before making a scheduling decision. In this step, we will take a look at some of those other factors.

Using the output from the previous step, find an allocation that has been placed
on a node in `dc2` and use the nomad [alloc status][alloc status] command with
the [verbose][verbose] option to obtain detailed scoring information on it. In
this example, we will use the allocation ID `0dfcf0ba` (your allocation IDs will
be different).

```shell
$ nomad alloc status -verbose 0dfcf0ba
``` 
The resulting output will show the `Placement Metrics` section at the bottom.

```shell
...
Placement Metrics
Node                                  binpack  job-anti-affinity  node-reschedule-penalty  node-affinity  final score
6b6e9518-d2a4-82c8-af3b-6805c8cdc29c  0.33     0                  0                        1              0.665
3dea0188-ae06-ad98-64dd-a761ab2b1bf3  0.33     0                  0                        0              0.33
3592943e-67e4-461f-d888-d5842372a4d4  0.33     0                  0                        0              0.33
```

Note that the results from the `binpack`, `job-anti-affinity`,
`node-reschedule-penalty`, and `node-affinity` columns are combined to produce the
numbers listed in the `final score` column for each node. The Nomad scheduler
uses the final score for each node in deciding where to make placements.

## Next Steps

Experiment with the weight provided in the `affinity` stanza (the value can be
from -100 through 100) and observe how the final score given to each node
changes (use the `nomad alloc status` command as shown in the previous step).

[affinity-stanza]: /docs/job-specification/affinity.html
[alloc status]: /docs/commands/alloc/status.html
[attributes]: /docs/runtime/interpolation.html#node-variables- 
[constraint]: /docs/job-specification/constraint.html
[client-metadata]: /docs/configuration/client.html#meta
[node-status]: /docs/commands/node/status.html
[scheduling]: /docs/internals/scheduling/scheduling.html
[verbose]: /docs/commands/alloc/status.html#verbose
[weight]: /docs/job-specification/affinity.html#weight

