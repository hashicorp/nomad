---
layout: "docs"
page_title: "volume Stanza - Job Specification"
sidebar_current: "docs-job-specification-volume"
description: |-
   The "volume" stanza allows the group to specify that it requires a given volume
   from the cluster. Nomad will automatically handle ensuring that the volume is
   available and mounted into the task.
---

# `volume` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> **volume**</code>
    </td>
  </tr>
</table>

The `volume` stanza allows the group to specify that it requires a given volume
from the cluster.

The key of the stanza is the name of the volume as it will be exposed to task
configuration.

```hcl
job "docs" {
  group "example" {
    volume "certs" {
      type      = "host"
      source    = "ca-certificates"
      read_only = true
    }
  }
}
```

The Nomad server will ensure that the allocations are only scheduled on hosts
that have a set of volumes that meet the criteria specified in the `volume`
stanzas.

The Nomad client will make the volumes available to tasks according to the
[volume_mount][volume_mount] stanza in the `task` configuration.

## `volume` Parameters

- `type` `(string: "")` - Specifies the type of a given volume. Currently the
  only possible volume type is `"host"`.

- `source` `(string: <required>)` - The name of the volume to request. When using
  `host_volume`'s this should match the published name of the host volume.

- `read_only` `(bool: false)` - Specifies that the group only requires read only
  access to a volume and is used as the default value for the `volume_mount ->
  read_only` configuration. This value is also used for validating `host_volume`
  ACLs and for scheduling when a matching `host_volume` requires `read_only`
  usage.

[volume_mount]: /docs/job-specification/volume_mount.html "Nomad volume_mount Job Specification"
