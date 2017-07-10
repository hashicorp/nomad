---
layout: "docs"
page_title: "ephemeral_disk Stanza - Job Specification"
sidebar_current: "docs-job-specification-ephemeral_disk"
description: |-
  The "ephemeral_disk" stanza describes the ephemeral disk requirements of the
  group. Ephemeral disks can be marked as sticky and support live data
  migrations.
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


The `ephemeral_disk` stanza describes the ephemeral disk requirements of the
group. Ephemeral disks can be marked as sticky and support live data migrations.
All tasks in this group will share the same ephemeral disk.

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

- `migrate` `(bool: false)` - When `sticky` is true, this specifies that the
  Nomad client should make a best-effort attempt to migrate the data from a
  remote machine if placement cannot be made on the original node. During data
  migration, the task will block starting until the data migration has completed.

- `size` `(int: 300)` - Specifies the size of the ephemeral disk in MB.  The
  current Nomad ephemeral storage implementation does not enforce this limit;
  however, it is used during job placement.

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
