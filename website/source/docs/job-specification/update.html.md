---
layout: "docs"
page_title: "update Stanza - Job Specification"
sidebar_current: "docs-job-specification-update"
description: |-
  The "update" stanza specifies the group's update strategy. The update strategy
  is used to control things like rolling upgrades and canary deployments. If
  omitted, rolling updates and canaries are disabled.
---

# `update` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> **update**</code>
    </td>
    <td>
      <code>job -> group -> **update**</code>
    </td>
  </tr>
</table>

The `update` stanza specifies the group's update strategy. The update strategy
is used to control things like rolling upgrades and canary deployments. If
omitted, rolling updates and canaries are disabled. If specified at the job
level, the configuration will apply to all groups within the job. If multiple
`update` stanzas are specified, they are merged with the group stanza taking the
highest precedence and then the job.

```hcl
job "docs" {
  update {
    max_parallel     = 3
    health_check     = "checks"
    min_healthy_time = "10s"
    healthy_deadline = "10m"
    auto_revert      = true
    canary           = 1
    stagger          = "30s"
  }
}
```

~> For `system` jobs, only `max_parallel` and `stagger` are enforced. The job is
updated at a rate of `max_parallel`, waiting `stagger` duration before the next
set of updates. The `system` scheduler will be updated to support the new
`update` stanza in a future release.

## `update` Parameters

- `max_parallel` `(int: 0)` - Specifies the number of task groups that can be
  updated at the same time.

- `health_check` `(string: "checks")` - Specifies the mechanism in which
  allocations health is determined. The potential values are:

  - "checks" - Specifies that the allocation should be considered healthy when
    all of its tasks are running and their associated [checks][] are healthy,
    and unhealthy if any of the tasks fail or not all checks become healthy.
    This is a superset of "task_states" mode.

  - "task_states" - Specifies that the allocation should be considered healthy when
    all its tasks are running and unhealthy if tasks fail.

  - "manual" - Specifies that Nomad should not automatically determine health
    and that the operator will specify allocation health using the [HTTP
    API](/api/deployments.html#set-allocation-health-in-deployment).

- `min_healthy_time` `(string: "10s")` - Specifies the minimum time the
  allocation must be in the healthy state before it is marked as healthy and
  unblocks further allocations from being updated. This is specified using a
  label suffix like "30s" or "15m".

- `healthy_deadline` `(string: "5m")` - Specifies the deadline in which the
  allocation must be marked as healthy after which the allocation is
  automatically transitioned to unhealthy. This is specified using a label
  suffix like "2m" or "1h".

- `auto_revert` `(bool: false)` - Specifies if the job should auto-revert to the
  last stable job on deployment failure. A job is marked as stable if all the
  allocations as part of its deployment were marked healthy.

- `canary` `(int: 0)` - Specifies that changes to the job that would result in
  destructive updates should create the specified number of canaries without
  stopping any previous allocations. Once the operator determines the canaries
  are healthy, they can be promoted which unblocks a rolling update of the
  remaining allocations at a rate of `max_parallel`.

- `stagger` `(string: "30s")` - Specifies the delay between migrating
  allocations off nodes marked for draining. This is specified using a label
  suffix like "30s" or "1h".

## `update` Examples

The following examples only show the `update` stanzas. Remember that the
`update` stanza is only valid in the placements listed above.

### Parallel Upgrades Based on Checks

This example performs 3 upgrades at a time and requires the allocations be
healthy for a minimum of 30 seconds before continuing the rolling upgrade. Each
allocation is given at most 2 minutes to determine its health before it is
automatically marked unhealthy and the deployment is failed.

```hcl
update {
  max_parallel     = 3
  min_healthy_time = "30s"
  healthy_deadline = "2m"
}
```

### Parallel Upgrades Based on Task State

This example is the same as the last but only requires the tasks to be healthy
and does not require registered service checks to be healthy.

```hcl
update {
  max_parallel     = 3
  min_healthy_time = "30s"
  healthy_deadline = "2m"
  health_check     = "task_states"
}
```

### Canary Upgrades

This example creates a canary allocation when the job is updated. The canary is
created without stopping any previous allocations from the job and allows
operators to determine if the new version of the job should be rolled out. 

```hcl
update {
  canary       = 1
  max_parallel = 3
}
```

Once the operator has determined the new job should be deployed, the deployment
can be promoted and a rolling update will occur performing 3 updates at a time
until the remainder of the groups allocations have been rolled to the new
version.

```text
# Promote the canaries for the job.
$ nomad job promote <job-id>
```

### Blue/Green Upgrades

By setting the canary count equal to that of the task group, blue/green
deployments can be achieved. When a new version of the job is submitted, instead
of doing a rolling upgrade of the existing allocations, the new version of the
group is deployed along side the existing set. While this duplicates the
resources required during the upgrade process, it allows very safe deployments
as the original version of the group is untouched.

```hcl
group "api-server" {
    count = 3

    update {
      canary       = 3
      max_parallel = 3
    }
    ...
}
```

Once the operator is satisfied that the new version of the group is stable, the
group can be promoted which will result in all allocations for the old versions
of the group to be shutdown. This completes the upgrade from blue to green, or
old to new version.

```text
# Promote the canaries for the job.
$ nomad job promote <job-id>
```

### Serial Upgrades

This example uses a serial upgrade strategy, meaning exactly one task group will
be updated at a time. The allocation must be healthy for the default
`min_healthy_time` of 10 seconds.

```hcl
update {
  max_parallel = 1
}
```

### Upgrade Stanza Inheritance

This example shows how inheritance can simplify the job when there are multiple
task groups.

```hcl
job "example" {
  ...

  update {
    max_parallel     = 2
    health_check     = "task_states"
    healthy_deadline = "10m"
  }

  group "one" {
    ...

    update {
      canary = 1      
    }
  }

  group "two" {
    ...

    update {
      min_healthy_time = "3m" 
    }
  }
}
```

By placing the shared parameters in the job's update stanza, each groups update
stanza may be kept to a minimum. The merged update stanzas for each group
becomes:

```hcl
group "one" {
  update {
    canary           = 1
    max_parallel     = 2
    health_check     = "task_states"
    healthy_deadline = "10m"
  }
}

group "two" {
  update {
    min_healthy_time = "3m" 
    max_parallel     = 2
    health_check     = "task_states"
    healthy_deadline = "10m"
  }
}
```

[checks]: /docs/job-specification/service.html#check-parameters "Nomad check Job Specification"
