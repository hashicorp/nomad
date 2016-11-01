---
layout: "docs"
page_title: "update Stanza - Job Specification"
sidebar_current: "docs-job-specification-update"
description: |-
  The "update" stanza specifies the job update strategy. The update strategy is
  used to control things like rolling upgrades. If omitted, rolling updates are
  disabled.
---

# `update` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> **update**</code>
    </td>
  </tr>
</table>

The `update` stanza specifies the job update strategy. The update strategy is
used to control things like rolling upgrades. If omitted, rolling updates are
disabled.

```hcl
job "docs" {
  update {
    max_parallel = 3
    stagger      = "30s"
  }
}
```

## `update` Parameters

- `max_parallel` `(int: 0)` - Specifies the number of tasks that can be updated
  at the same time.

- `stagger` `(string: "0ms")` - Specifies the delay between sets of updates.
  This is specified using a label suffix like "30s" or "1h".

## `update` Examples

The following examples only show the `update` stanzas. Remember that the
`update` stanza is only valid in the placements listed above.

### Serial Upgrades

This example uses a serial upgrade strategy, meaning exactly one task will be
updated at a time, waiting 60 seconds until the next task is upgraded.

```hcl
update {
  max_parallel = 1
  stagger      = "60s"
}
```

### Parallel Upgrades

This example performs 10 upgrades at a time, waiting 30 seconds before moving on
to the next batch:

```hcl
update {
  max_parallel = 10
  stagger      = "30s"
}
```
