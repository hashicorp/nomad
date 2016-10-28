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

- `count` `(int: 1)` - Specifies the number of the task groups that should
  be running under this group. This value must be non-negative.

- `constraint` <code>([Constraint][]: nil)</code> -
  This can be provided multiple times to define additional constraints.

- `restart` <code>([Restart][]: nil)</code> - Specifies the restart policy for
  all tasks in this group. If omitted, a default policy exists for each job
  type.

- `task` <code>([Task][]: required)</code> - Specifies one or more tasks to run
  within this group. This can be specified multiple times, to add a task as part
  of the group.

- `meta` <code>([Meta][]: nil)</code> - Specifies a key-value map that annotates
  with user-defined metadata.

## `group` Examples

The following examples only show the `group` stanzas. Remember that the `group`
Remember that the `group` stanza is only valid in the placements listed above.

### Specifying Count

This example specifies that 5 instances of the tasks within this group should be
running:

```hcl
group "example" {
  count = 5
}
```

### Task with Constraint

This example shows an abbreviated task with a constraint in the group. This will
restrict the task (and any other tasks in this group) to 64-bit operating
systems.

```hcl
group "example" {
  constraint {
    attribute = "${attr.arch}"
    value     = "amd64"
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

[task]: /docs/job-specification/task.html "Nomad task Specification"
[job]: /docs/job-specification/job.html "Nomad job Specification"
[constraint]: /docs/job-specification/constraint.html "Nomad constraint Specification"
[meta]: /docs/job-specification/meta.html "Nomad meta Specification"
[restart]: /docs/job-specification/restart.html "Nomad restart Specification"
