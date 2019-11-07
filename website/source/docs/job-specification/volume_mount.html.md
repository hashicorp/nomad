---
layout: "docs"
page_title: "volume_mount Stanza - Job Specification"
sidebar_current: "docs-job-specification-volume_mount"
description: |-
   The "volume_mount" stanza allows the task to specify where a group "volume"
   should be mounted.
---

# `volume_mount` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> task -> **volume_mount**</code>
    </td>
  </tr>
</table>

The `volume_mount` stanza allows the task to specify how a group
[`volume`][volume] should be mounted into the task.

```hcl
job "docs" {
  group "example" {
    volume "certs" {
      type = "host"
      read_only = true
      source = "ca-certificates"
    }

    task "example" {
      volume_mount {
        volume      = "certs"
        destination = "/etc/ssl/certs"
      }
    }
  }
}
```

The Nomad client will make the volumes available to tasks according to this
configuration, and it will fail the allocation if the client configuration
updates to remove a volume that it depends on.

## `volume_mount` Parameters

- `volume` `(string: "")` - Specifies the group volume that the mount is going
  to access.

- `destination` `(string: "")` - Specifies where the volume should be mounted
  inside the tasks container.

- `read_only` `(bool: false)` - When a group volume is writeable, you may
  specify that it is `read_only` on a per mount level using the `read_only`
  option here.

[volume]: /docs/job-specification/volume.html "Nomad volume Job Specification"
