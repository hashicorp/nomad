---
layout: "docs"
page_title: "env Stanza - Job Specification"
sidebar_current: "docs-job-specification-env"
description: |-
  The "env" stanza configures a list of environment variables to populate the
  task's environment before starting.
---

# `env` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> task -> **env**</code>
    </td>
  </tr>
</table>

The `env` stanza configures a list of environment variables to populate the
task's environment before starting.

```hcl
job "docs" {
  group "example" {
    task "server" {
      env {
        my_key = "my-value"
      }
    }
  }
}
```

## `env` Parameters

The "parameters" for the `env` stanza can be any key-value. The keys and values
are both of type `string`, but they can be specified as other types. They will
automatically be converted to strings. Invalid characters such as dashes (`-`)
will be converted to underscores.

## `env` Examples

The following examples only show the `env` stanzas. Remember that the
`env` stanza is only valid in the placements listed above.

### Coercion

This example shows the different ways to specify key-value pairs. Internally,
these values will be stored as their string representation. No type information
is preserved.

```hcl
env {
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
env {
  NODE_CLASS = "${nomad.class}"
}
```

### Dynamic Environment Variables

Nomad also supports populating dynamic environment variables from data stored in
HashiCorp Consul and Vault. To use this feature please see the documentation on
the [`template` stanza][template-env].

[interpolation]: /docs/runtime/interpolation.html "Nomad interpolation"
[template-env]: /docs/job-specification/template.html#environment-variables "Nomad template Stanza"
