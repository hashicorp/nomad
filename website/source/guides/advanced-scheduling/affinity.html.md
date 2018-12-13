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


[affinity-stanza]: /docs/job-specification/affinity.html
[attributes]: /docs/runtime/interpolation.html#node-variables- 
[constraint]: /docs/job-specification/constraint.html
[client-metadata]: /docs/configuration/client.html#meta
[scheduling]: /docs/internals/scheduling.html
[weight]: /docs/job-specification/affinity.html#weight

