---
layout: "docs"
page_title: "migrate Stanza - Job Specification"
sidebar_current: "docs-job-specification-migrate"
description: |-
  The "migrate" stanza specifies the group's migrate strategy. The migrate
  strategy is used to control the job's behavior when it is being migrated off
  of a draining node.
---

# `migrate` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> **migrate**</code>
      <br>
      <code>job -> group -> **migrate**</code>
    </td>
  </tr>
</table>

The `migrate` stanza specifies the group's strategy for migrating off of
[draining][drain] nodes. If omitted, a default migration strategy is applied.
If specified at the job level, the configuration will apply to all groups
within the job. Only service jobs with a count greater than 1 support migrate
stanzas.

```hcl
job "docs" {
  migrate {
    max_parallel     = 1
    health_check     = "checks"
    min_healthy_time = "10s"
    healthy_deadline = "5m"
  }
}
```

When one or more nodes are draining, only `max_parallel` allocations will be
stopped at a time. Node draining will not continue until replacement
allocations have been healthy for their `min_healthy_time` or
`healthy_deadline` is reached.

Note that a node's drain [deadline][deadline] will override the `migrate`
stanza for allocations on that node. The `migrate` stanza is for job authors to
define how their services should be migrated, while the node drain deadline is
for system operators to put hard limits on how long a drain may take.

See the [Workload Migration Guide](/guides/operations/node-draining.html) for details
on node draining.

## `migrate` Parameters

- `max_parallel` `(int: 1)` - Specifies the number of allocations that can be
  migrated at the same time. This number must be less than the total
  [`count`][count] for the group as `count - max_parallel` will be left running
  during migrations.

- `health_check` `(string: "checks")` - Specifies the mechanism in which
  allocations health is determined. The potential values are:

  - "checks" - Specifies that the allocation should be considered healthy when
    all of its tasks are running and their associated [checks][checks] are
    healthy, and unhealthy if any of the tasks fail or not all checks become
    healthy.  This is a superset of "task_states" mode.

  - "task_states" - Specifies that the allocation should be considered healthy when
    all its tasks are running and unhealthy if tasks fail.

- `min_healthy_time` `(string: "10s")` - Specifies the minimum time the
  allocation must be in the healthy state before it is marked as healthy and
  unblocks further allocations from being migrated. This is specified using a
  label suffix like "30s" or "15m".

- `healthy_deadline` `(string: "5m")` - Specifies the deadline in which the
  allocation must be marked as healthy after which the allocation is
  automatically transitioned to unhealthy. This is specified using a label
  suffix like "2m" or "1h".


[checks]: /docs/job-specification/service.html#check-parameters
[count]: /docs/job-specification/group.html#count
[drain]: /docs/commands/node/drain.html
[deadline]: /docs/commands/node/drain.html#deadline
