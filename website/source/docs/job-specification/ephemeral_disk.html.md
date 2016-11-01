---
layout: "docs"
page_title: "ephemeral_disk Stanza - Job Specification"
sidebar_current: "docs-job-specification-ephemeral_disk"
description: |-
  The "ephemeral_disk" stanza instructs Nomad to utilize an ephemeral disk
  instead of a hard disk requirement, and can also enable sticky volumes and
  live data migrations.
---

# `ephemeral_disk` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> **ephemeral_disk**</code>
    </td>
  </tr>
</table>

The `ephemeral_disk` stanza instructs Nomad to utilize an ephemeral disk instead
of a hard disk requirement. Clients using this stanza should not specify disk
requirements in the [resources stanza][resources] of the task. All tasks in this
group will share the same ephemeral disk.

```hcl
job "docs" {
  group "example" {
    ephemeral_disk {
      migrate = true
      size    = "500"
      sticky  = true
    }
  }
}
```

## `ephemeral_disk` Parameters

- `migrate` `(bool: false)` - Specifies that the Nomad client should make a
  best-effort attempt to migrate the data from a remote machine if placement
  should fail. During data migration, the task will block starting until the
  data migration has completed.

- `size` `(int: 300)` - Specifies the size of the ephemeral disk in MB.

- `sticky` `(bool: false)` - Specifies that Nomad should make a best-effort
  attempt to place the updated allocation on the same machine. This will move
  the `local/` and `alloc/data` directories to the new allocation.

## `ephemeral_disk` Examples

The following examples only show the `ephemeral_disk` stanzas. Remember that the
`ephemeral_disk` stanza is only valid in the placements listed above.

### Sticky Volumes

This example shows enabling sticky volumes with Nomad using ephemeral disks:

```hcl
ephemeral_disk {
  sticky = true
}
```

[resources]: /docs/job-specification/resources.html "Nomad resources Job Specification"
