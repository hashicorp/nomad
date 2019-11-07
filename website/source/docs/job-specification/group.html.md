---
layout: "docs"
page_title: "group Stanza - Job Specification"
sidebar_current: "docs-job-specification-group"
description: |-
  The "group" stanza defines a series of tasks that should be co-located on the
  same Nomad client. Any task within a group will be placed on the same client.
---

# `group` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> **group**</code>
    </td>
  </tr>
</table>

The `group` stanza defines a series of tasks that should be co-located on the
same Nomad client. Any [task][] within a group will be placed on the same
client.

```hcl
job "docs" {
  group "example" {
    # ...
  }
}
```

## `group` Parameters

- `constraint` <code>([Constraint][]: nil)</code> -
  This can be provided multiple times to define additional constraints.

- `affinity` <code>([Affinity][]: nil)</code> - This can be provided
    multiple times to define preferred placement criteria.

- `spread` <code>([Spread][spread]: nil)</code> - This can be provided
  multiple times to define criteria for spreading allocations across a
  node attribute or metadata. See the
  [Nomad spread reference](/docs/job-specification/spread.html) for more details.

- `count` `(int: 1)` - Specifies the number of the task groups that should
  be running under this group. This value must be non-negative.

- `ephemeral_disk` <code>([EphemeralDisk][]: nil)</code> - Specifies the
  ephemeral disk requirements of the group. Ephemeral disks can be marked as
  sticky and support live data migrations.

- `meta` <code>([Meta][]: nil)</code> - Specifies a key-value map that annotates
  with user-defined metadata.

- `migrate` <code>([Migrate][]: nil)</code> - Specifies the group strategy for
  migrating off of draining nodes. Only service jobs with a count greater than
  1 support migrate stanzas.

- `reschedule` <code>([Reschedule][]: nil)</code> - Allows to specify a
  rescheduling strategy. Nomad will then attempt to schedule the task on another
  node if any of the group allocation statuses become "failed".

- `restart` <code>([Restart][]: nil)</code> - Specifies the restart policy for
  all tasks in this group. If omitted, a default policy exists for each job
  type, which can be found in the [restart stanza documentation][restart].

- `task` <code>([Task][]: <required>)</code> - Specifies one or more tasks to run
  within this group. This can be specified multiple times, to add a task as part
  of the group.

- `vault` <code>([Vault][]: nil)</code> - Specifies the set of Vault policies
  required by all tasks in this group. Overrides a `vault` block set at the
  `job` level.

- `volume` <code>([Volume][]: nil)</code> - Specifies the volumes that are
  required by tasks within the group.

## `group` Examples

The following examples only show the `group` stanzas. Remember that the
`group` stanza is only valid in the placements listed above.

### Specifying Count

This example specifies that 5 instances of the tasks within this group should be
running:

```hcl
group "example" {
  count = 5
}
```

### Tasks with Constraint

This example shows two abbreviated tasks with a constraint on the group. This
will restrict the tasks to 64-bit operating systems.

```hcl
group "example" {
  constraint {
    attribute = "${attr.cpu.arch}"
    value     = "amd64"
  }

  task "cache" {
    # ...
  }

  task "server" {
    # ...
  }
}
```

### Metadata

This example show arbitrary user-defined metadata on the group:

```hcl
group "example" {
  meta {
    "my-key" = "my-value"
  }
}
```

[task]: /docs/job-specification/task.html "Nomad task Job Specification"
[job]: /docs/job-specification/job.html "Nomad job Job Specification"
[constraint]: /docs/job-specification/constraint.html "Nomad constraint Job Specification"
[spread]: /docs/job-specification/spread.html "Nomad spread Job Specification"
[affinity]: /docs/job-specification/affinity.html "Nomad affinity Job Specification"
[ephemeraldisk]: /docs/job-specification/ephemeral_disk.html "Nomad ephemeral_disk Job Specification"
[meta]: /docs/job-specification/meta.html "Nomad meta Job Specification"
[migrate]: /docs/job-specification/migrate.html "Nomad migrate Job Specification"
[reschedule]: /docs/job-specification/reschedule.html "Nomad reschedule Job Specification"
[restart]: /docs/job-specification/restart.html "Nomad restart Job Specification"
[vault]: /docs/job-specification/vault.html "Nomad vault Job Specification"
[volume]: /docs/job-specification/volume.html "Nomad volume Job Specification"
