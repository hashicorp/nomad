---
layout: "docs"
page_title: "meta Stanza - Job Specification"
sidebar_current: "docs-job-specification-meta"
description: |-
  The "meta" stanza allows for user-defined arbitrary key-value pairs.
---

# `meta` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> **meta**</code>
      <br>
      <code>job -> group -> **meta**</code>
      <br>
      <code>job -> group -> task -> **meta**</code>
    </td>
  </tr>
</table>

The `meta` stanza allows for user-defined arbitrary key-value pairs. It is
possible to use the `meta` stanza at the [job][], [group][], or [task][] level.

```hcl
job "docs" {
  meta {
    my-key = "my-value"
  }

  group "example" {
    meta {
      my-key = "my-value"
    }

    task "server" {
      meta {
        my-key = "my-value"
      }
    }
  }
}
```

Metadata is merged up the job specification, so metadata defined at the job
level applies to all groups and tasks within that job. Metadata defined at the
group layer applies to all tasks within that group.

## `meta` Parameters

The "parameters" for the `meta` stanza can be any key-value. The keys and values
are both of type `string`, but they can be specified as other types. They will
automatically be converted to strings.

## `meta` Examples

The following examples only show the `meta` stanzas. Remember that the
`meta` stanza is only valid in the placements listed above.

### Coercion

This example shows the different ways to specify key-value pairs. Internally,
these values will be stored as their string representation. No type information
is preserved.

```hcl
meta {
  key = "true"
  key = true

  "key" = true

  key = 1.4
  key = "1.4"
}
```

### Interpolation

This example shows using [Nomad interpolation][interpolation] to populate
environment variables.

```hcl
meta {
  class = "${nomad.class}"
}
```

[job]: /docs/job-specification/job.html "Nomad job Job Specification"
[group]: /docs/job-specification/group.html "Nomad group Job Specification"
[task]: /docs/job-specification/task.html "Nomad task Job Specification"
[interpolation]: /docs/runtime/interpolation.html "Nomad interpolation"
