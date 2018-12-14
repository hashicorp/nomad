---
layout: "guides"
page_title: "Affinity"
sidebar_current: "guides-advanced-scheduling"
description: |-
   The affinity stanza allows operators to express placement preference for a set of nodes. Affinities may be expressed on attributes or client metadata.  
---

# Expressing Job Placement Preferences with Affinities

The [affinity][affinity-stanza] stanza allows operators to express placement preferences for their jobs on particular types of nodes. It is important to remember that there is a key difference between the [constraint][constraint] stanza and the affinity stanza: while the `constraint` stanza strictly filters where jobs are run based on [attributes][attributes] and [client metadata][client-metadata], the `affinity` stanza is more flexible and allows the job to run on resources that fit best after making an attempt to match the Nomad operator's specifications. This is done in conjunction with scoring based on the Nomad scheduler's bin packing algorithm which you can read more about [here][scheduling].

## Reference Material

- The [affinity][affinity-stanza] stanza documentation
- [Scheduling][scheduling] with Nomad

## Estimated Time to Complete

20 minutes

## Challenge

Think of a scenario where a Nomad operator needs the flexibility to express placement preferences for a critical job but still have the scheduler run the job on appropriate nodes anywhere if the desired resources are not available.

## Solution

Specify an affinity with the proper [weight][weight] so that the Nomad scheduler
can find the best nodes on which to place your job. The weight of the affinity
will be factored in with the results of the Nomad scheduler's bin packing algorithm (which is used to optimize the resource utilization and density of applications) to find the best fit.

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

If everything worked correctly, you should be able to run the `nomad node
status` command and see that one of your nodes is now in datacenter `dc2`.

```shell
$ nomad node status
ID        DC   Name              Class   Drain  Eligibility  Status
335de832  dc2  ip-172-31-50-163  <none>  false  eligible     ready
1cdde5bc  dc1  ip-172-31-48-23   <none>  false  eligible     ready
4bbb2aa7  dc1  ip-172-31-59-0    <none>  false  eligible     ready
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
value for the [attribute][attributes] `${node.datacenter}`. We used the value
`100` for the [weight][weight] which will cause the Nomad schedular to rank nodes
in datacenter `dc2` with a higher score. Keep in mind that weights can range
from -100 to 100, inclusive. Negative weights serve as anti-affinities.

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

Note that two of the allocations have been placed on node `6b6e9518`. This is
the node we configured to be in datacenter `dc2`. The Nomad scheduler has leaned
towards placing the majority of our workload on this node because of the
affinity we specified. All of the workload has not been placed on this node
because the Nomad scheduler still factors in other components into the overall
scoring when making placements. We will take a detailed look at the scoring in the next few steps.



[affinity-stanza]: /docs/job-specification/affinity.html
[attributes]: /docs/runtime/interpolation.html#node-variables- 
[constraint]: /docs/job-specification/constraint.html
[client-metadata]: /docs/configuration/client.html#meta
[scheduling]: /docs/internals/scheduling.html
[weight]: /docs/job-specification/affinity.html#weight

