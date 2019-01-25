---
layout: "docs"
page_title: "Preemption"
sidebar_current: "docs-internals-scheduling-preemption"
description: |-
  Learn about how preemption works in Nomad.
---

# Preemption

Preemption allows Nomad to kill existing allocations in order to place allocations for a higher priority job.
The evicted allocation is temporary displaced until the cluster has capacity to run it. This allows operators to
run high priority jobs even under resource contention across the cluster.


~> **Advanced Topic!** This page covers technical details of Nomad. You do not
~> need to understand these details to effectively use Nomad. The details are
~> documented here for those who wish to learn about them without having to
~> go spelunking through the source code.

# Preemption in Nomad

Every job in Nomad has a priority associated with it. Priorities impact scheduling at the evaluation and planning
stages by sorting the respective queues accordingly (higher priority jobs get moved ahead in the queues).

Prior to Nomad 0.9, when a cluster is at capacity, any allocations that result from a newly scheduled or updated
job remain in the pending state until sufficient resources become available - regardless of the defined priority.
This leads to priority inversion, where a low priority task can prevent high priority tasks from completing.

Nomad 0.9 brings preemption capabilities to system jobs. The Nomad scheduler will evict lower priority running allocations
to free up capacity for new allocations resulting from relatively higher priority jobs, sending evicted allocations back
into the plan queue.

# Details

Preemption is enabled by default in Nomad 0.9. Operators can use the [scheduler config](/api/operator.html#update-scheduler-configuration) API endpoint to disable preemption.

Nomad uses the [job priority](/docs/job-specification/job.html#priority) field to determine what running allocations can be preempted.
In order to prevent a cascade of preemptions due to jobs close in priority being preempted, only allocations from jobs with a priority
delta of more than 10 from the job needing placement are eligible for preemption.

For example, consider a node with the following distribution of allocations:

| Job           | Priority      | Allocations  | Total Used capacity |
| ------------- |-------------| --------------   |------------
| cache         | 70 | a6        |  2 GB Memory, 0.5 GB Disk, 1 CPU
| batch-analytics|  50     |   a4, a5       | <1 GB Memory, 0.5 GB Disk, 0.5 CPU>, <1 GB Memory, 0.5 GB Disk, 0.5 CPU>
| email-marketing |   20   |    a1, a2        | <0.5 GB Memory, 0.8 GB Disk>, <0.5 GB Memory, 0.2 GB Disk>

If a job `webapp` with priority `75` needs placement on the above node, only allocations from `batch-analytics` and `email-marketing` are considered
eligible to be preempted because they are of a lower priority. Allocations from the `cache` job will never be preempted because its priority value `70`
is lesser than the required delta of `10`.

Allocations are selected starting from the lowest priority, and scored according
to how closely they fit the job's required capacity. For example, if the `75` priority job needs 1GB disk and 2GB memory, Nomad will preempt
allocations `a1`, `a2` and `a4` to satisfy those requirements.

# Preemption Visibility

Operators can use the [allocation API](/api/allocations.html#read-allocation) or the `alloc status` command to get visibility into
whether an allocation has been preempted. Preempted allocations will have their DesiredStatus set to “evict”. The `Allocation` object
in the API also has two additional fields related to preemption.

- `PreemptedAllocs` - This field is set on an allocation that caused preemption. It contains the allocation ids of allocations
  that were preempted to place this allocation. In the above example, allocations created for the job `webapp` will have the values
  `a1`, `a2` and `a4` set.
- `PreemptedByAllocID` - This field is set on allocations that were preempted by the scheduler. It contains the allocation ID of the allocation
  that preempted it. In the above example, allocations `a1`, `a2` and `a4` will have this field set to the ID of the allocation from the job `webapp`.

# Integration with Nomad plan

`nomad plan` allows operators to dry run the scheduler. If the scheduler determines that
preemption is necessary to place the job, it shows additional information in the CLI output for
`nomad plan` as seen below.

```sh
$ nomad plan example.nomad

+ Job: "test"
+ Task Group: "test" (1 create)
  + Task: "test" (forces create)

Scheduler dry-run:
- All tasks successfully allocated.

Preemptions:

Alloc ID                              Job ID    Task Group
ddef9521                              my-batch   analytics
ae59fe45                              my-batch   analytics
```

Note that, the allocations shown in the `nomad plan` output above
are not guaranteed to be the same ones picked when running the job later.
They provide the operator a sample of the type of allocations that could be preempted.

[Omega]: https://research.google.com/pubs/pub41684.html
[Borg]: https://research.google.com/pubs/pub43438.html
[img-data-model]: /assets/images/nomad-data-model.png
[img-eval-flow]: /assets/images/nomad-evaluation-flow.png
