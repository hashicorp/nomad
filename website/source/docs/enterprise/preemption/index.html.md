---
layout: "docs"
page_title: "Nomad Enterprise Preemption"
sidebar_current: "docs-enterprise-preemption"
description: |-
  Nomad Enterprise preemption capabilities enable the scheduler to temporarily
  evict lower priority allocations for service and batch jobs so that
  higher priority allocations can be placed. 
---

# Nomad Enterprise Preemption

When a Nomad cluster is at capacity for a given set of placement constraints, any allocations 
that result from a newly scheduled service or batch job will remain in the pending state until 
sufficient resources become available - regardless of the defined priority.

[Preemption](/docs/internals/scheduling/preemption.html) capabilities in 
[Nomad Enterprise](https://www.hashicorp.com/go/nomad-enterprise) enable the scheduler to temporarily 
evict lower [priority](/docs/job-specification/job.html#priority) allocations from service and 
batch jobs so that the allocations from higher priority jobs can be placed. This behavior 
ensures that critical workloads can run when resources are limited or when partial outages require 
workloads to be rescheduled across a smaller set of client nodes.

See the [Preemption internals documentation](/docs/internals/scheduling/preemption.html) for a 
more detailed overview. Preemption for service and batch jobs can be enabled using the [scheduler config API endpoint](/api/operator.html#update-scheduler-configuration).

Click [here](https://www.hashicorp.com/go/nomad-enterprise) to set up a demo or 
request a trial of Nomad Enterprise.