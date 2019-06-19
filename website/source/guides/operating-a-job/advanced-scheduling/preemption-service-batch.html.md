---
layout: "guides"
page_title: "Preemption (Service and Batch Jobs)"
sidebar_current: "guides-operating-a-job-preemption-service-batch"
description: |-
  The following guide walks the user through enabling and using preemption on
  service and batch jobs in Nomad Enterprise (0.9.3 and above). 
---

# Preemption for Service and Batch Jobs

Starting with Nomad 0.9, every job in Nomad has a [priority][priority]
associated with it. Preemption allows Nomad to evict current allocations with
newly scheduled allocations that have been assigned a higher priority. The
evicted allocation is temporarily displaced until the cluster has the capacity
to run it. This allows operators to run high priority jobs even under resource
contention across the cluster.

While Nomad 0.9 introduced preemption for [system][system-job] jobs, Nomad 0.9.3
[Enterprise][enterprise] additionally allows preemption for
[service][service-job] and [batch][batch-job] jobs.

## Reference Material

- [Preemption][preemption]
- [Nomad Enterprise Preemption][enterprise-preemption]

## Estimated Time to Complete

20 minutes

## Challenge

Consider a high-priority service application that needs to be deployed
immediately in your Nomad cluster but currently cannot be placed anywhere due to
a lack of resources on your nodes. Stop any allocations with a lower priority
and place them in a queue to be run later when resources become available.

## Solution

[Update][update-scheduler] the scheduler configuration to enable service job
preemption in your Nomad cluster. Assign the job you currently need to deploy a
[priority][priority] greater than any job you would like to evict and send into
a queue for later deployment. Deploy your job and ensure it is running.

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this
[repo](https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud)
to easily provision a sandbox environment. This guide will assume a cluster with
one server node and three client nodes. To simulate resource contention, the
nodes in this environment will each have 1 GB RAM (For AWS, you can choose the
[t2.micro][t2-micro] instance type). Remember that service and batch job
preemption require Nomad 0.9.3 [Enterprise][enterprise].

-> **Please Note:** This guide is for demo purposes and is only using a single
server node. In a production cluster, 3 or 5 server nodes are recommended.

## Steps

### Step 1:


### Step 2: 


### Step 3: 


### Step 4: 


## Next Steps

[batch-job]: /docs/schedulers.html#batch
[enterprise]: /docs/enterprise/index.html
[enterprise-preemption]: /docs/enterprise/preemption/index.html
[preemption]: /docs/internals/scheduling/preemption.html
[priority]: /docs/job-specification/job.html#priority
[service-job]: /docs/schedulers.html#service
[system-job]: /docs/schedulers.html#system
[t2-micro]: https://aws.amazon.com/ec2/instance-types/
[update-scheduler]: /api/operator.html#update-scheduler-configuration