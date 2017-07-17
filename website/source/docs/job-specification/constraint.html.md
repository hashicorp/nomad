---
layout: "docs"
page_title: "constraint Stanza - Job Specification"
sidebar_current: "docs-job-specification-constraint"
description: |-
  The "constraint" stanza allows restricting the set of eligible nodes.
  Constraints may filter on attributes or metadata. Additionally constraints may
  be specified at the job, group, or task levels for ultimate flexibility.
---

# `constraint` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> **constraint**</code>
      <br>
      <code>job -> group -> **constraint**</code>
      <br>
      <code>job -> group -> task -> **constraint**</code>
    </td>
  </tr>
</table>

The `constraint` allows restricting the set of eligible nodes. Constraints may
filter on [attributes][interpolation] or [client metadata][client-meta].
Additionally constraints may be specified at the [job][job], [group][group], or
[task][task] levels for ultimate flexibility.

```hcl
job "docs" {
  # All tasks in this job must run on linux.
  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "example" {
    # All groups in this job should be scheduled on different hosts.
    constraint {
      operator  = "distinct_hosts"
      value     = "true"
    }

    task "server" {
      # All tasks must run where "my_custom_value" is greater than 3.
      constraint {
        attribute = "${meta.my_custom_value}"
        operator  = ">"
        value     = "3"
      }
    }
  }
}
```

Placing constraints at both the job level and at the group level is redundant
since constraints are applied hierarchically. The job constraints will affect
all groups (and tasks) in the job.

## `constraint` Parameters

- `attribute` `(string: "")` - Specifies the name or reference of the attribute
  to examine for the constraint. This can be any of the [Nomad interpolated
  values](/docs/runtime/interpolation.html#interpreted_node_vars).

- `operator` `(string: "=")` - Specifies the comparison operator. The ordering is
  compared lexically. Possible values include:

    ```text
    =
    !=
    >
    >=
    <
    <=
    regexp
    set_contains
    version
    ```

    For a detailed explanation of these values and their behavior, please see
    the [operator values section](#operator-values).

- `value` `(string: "")` - Specifies the value to compare the attribute against
  using the specified operation. This can be a literal value, another attribute,
  or any [Nomad interpolated
  values](/docs/runtime/interpolation.html#interpreted_node_vars).

### `operator` Values

This section details the specific values for the "operator" parameter in the
Nomad job specification for constraints. The operator is always specified as a
string, but the string can take on different values which change the behavior of
the overall constraint evaluation.

```hcl
constraint {
  operator = "..."
}
```

- `"distinct_hosts"` - Instructs the scheduler to not co-locate any groups on
  the same machine. When specified as a job constraint, it applies to all groups
  in the job. When specified as a group constraint, the effect is constrained to
  that group. This constraint can not be specified at the task level. Note that
  the `attribute` parameter should be omitted when using this constraint.

    ```hcl
    constraint {
      operator  = "distinct_hosts"
      value     = "true"
    }
    ```

    The constraint may also be specified as follows for a more compact
    representation:

    ```hcl
    constraint {
        distinct_hosts = true
    }
    ```

- `"distinct_property"` - Instructs the scheduler to select nodes that have a
  distinct value of the specified property for each allocation. When specified
  as a job constraint, it applies to all groups in the job. When specified as a
  group constraint, the effect is constrained to that group. This constraint can
  not be specified at the task level. Note that the `value` parameter should be
  omitted when using this constraint.

    ```hcl
    constraint {
      operator  = "distinct_property"
      attribute = "${meta.rack}"
    }
    ```

    The constraint may also be specified as follows for a more compact
    representation:

    ```hcl
    constraint {
        distinct_property = "${meta.rack}"
    }
    ```

- `"regexp"` - Specifies a regular expression constraint against the attribute.
  The syntax of the regular expressions accepted is the same general syntax used
  by Perl, Python, and many other languages. More precisely, it is the syntax
  accepted by RE2 and described at in the [Google RE2
  syntax](https://golang.org/s/re2syntax).

    ```hcl
    constraint {
      attribute = "..."
      operator  = "regexp"
      value     = "[a-z0-9]"
    }
    ```

- `"set_contains"` - Specifies a contains constraint against the attribute. The
  attribute and the list being checked are split using commas. This will check
  that the given attribute contains **all** of the specified elements.

    ```hcl
    constraint {
      attribute = "..."
      operator  = "set_contains"
      value     = "a,b,c"
    }
    ```

- `"version"` - Specifies a version constraint against the attribute. This
  supports a comma-separated list of constraints, including the pessimistic
  operator. For more examples please see the [go-version
  repository](https://github.com/hashicorp/go-version) for more specific
  examples.

    ```hcl
    constraint {
      attribute = "..."
      operator  = "version"
      value     = ">= 0.1.0, < 0.2"
    }
    ```

## `constraint` Examples

The following examples only show the `constraint` stanzas. Remember that the
`constraint` stanza is only valid in the placements listed above.

### Kernel Data

This example restricts the task to running on nodes which have a kernel version
higher than "3.19".

```hcl
constraint {
  attribute = "${attr.kernel.version}"
  operator  = "version"
  value     = "> 3.19"
}
```

### Distinct Property

A potential use case of the `distinct_property` constraint is to spread a
service with `count > 1` across racks to minimize correlated failure. Nodes can
be annotated with which rack they are on using [client
metadata][client-metadata] with values
such as "rack-12-1", "rack-12-2", etc. The following constraint would then
assure no two instances of the task group existed on the same rack.

```hcl
constraint {
  distinct_property = "${meta.rack}"
}
```

### Operating Systems

This example restricts the task to running on nodes that are running Ubuntu
14.04

```hcl
constraint {
  attribute = "${attr.os.name}"
  value     = "ubuntu"
}

constraint {
  attribute = "${attr.os.version}"
  value     = "14.04"
}
```

### Cloud Metadata

When possible, Nomad populates node attributes from the cloud environment. These
values are accessible as filters in constraints. This example constrains this
task to only run on nodes that are memory-optimized on AWS.

```hcl
constraint {
  attribute = "${attr.platform.aws.instance-type}"
  value     = "m4.xlarge"
}
```

### User-Specified Metadata

This example restricts the task to running on nodes where the binaries for
redis, cypress, and nginx are all cached locally. This particular example is
utilizing node [metadata][meta].

```hcl
constraint {
  attribute    = "${meta.cached_binaries}"
  set_contains = "redis,cypress,nginx"
}
```

[job]: /docs/job-specification/job.html "Nomad job Job Specification"
[group]: /docs/job-specification/group.html "Nomad group Job Specification"
[client-meta]: /docs/agent/configuration/client.html#meta "Nomad meta Job Specification"
[task]: /docs/job-specification/task.html "Nomad task Job Specification"
[interpolation]: /docs/runtime/interpolation.html "Nomad interpolation"
