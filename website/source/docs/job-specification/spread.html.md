---
layout: "docs"
page_title: "spread Stanza - Job Specification"
sidebar_current: "docs-job-specification-spread"
description: |-
  The "spread" stanza is used to spread placements across a certain node attributes such as datacenter.
  Spread may be specified at the job, group, or task levels for ultimate flexibility.
  More than one spread stanza may be specified with relative weights between each.
---

# `spread` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> **spread**</code>
      <br>
      <code>job -> group -> **spread**</code>
      <br>
      <code>job -> group -> task -> **spread**</code>
    </td>
  </tr>
</table>

The `spread` stanza allows operators to increase the failure tolerance of their
applications by specifying a node attribute that allocations should be spread
over. This allows operators to spread allocations over attributes such as
datacenter, availability zone, or even rack in a physical datacenter. By
default, when using spread the scheduler will attempt to place allocations
equally among the available values of the given target.


```hcl
job "docs" {
  # Spread allocations over all datacenter
  spread {
    attribute = "${node.datacenter}"
  }

  group "example" {
    # Spread allocations over each rack based on desired percentage
      spread {
        attribute = "${node.datacenter}"
        target "us-east1" {
          percent = 60
        }
        target "us-west1" {
          percent = 40
        }
      }
  }
}
```

Nodes are scored according to how closely they match the desired target percentage defined in the
spread stanza. Spread scores are combined with other scoring factors such as bin packing.

A job or task group can have more than one spread criteria, with weights to express relative preference.

Spread criteria are treated as a soft preference by the Nomad scheduler.
If no nodes match a given spread criteria, placement is still successful.

Spread may be expressed on [attributes][interpolation] or [client metadata][client-meta].
Additionally, spread may be specified at the [job][job] and [group][group] levels for ultimate flexibility.


## `spread` Parameters

- `attribute` `(string: "")` - Specifies the name or reference of the attribute
  to use. This can be any of the [Nomad interpolated
  values](/docs/runtime/interpolation.html#interpreted_node_vars).

- `target` <code>([target](#target-parameters): <required>)</code> - Specifies one or more target
   percentages for each value of the `attribute` in the spread stanza. If this is omitted,
   Nomad will spread allocations evenly across all values of the attribute.

- `weight` `(integer:0)` - Specifies a weight for the spread stanza. The weight is used
  during scoring and must be an integer between 0 to 100. Weights can be used
  when there is more than one spread or affinity stanza to express relative preference across them.

## `target` Parameters

- `value` `(string:"")` - Specifies a target value of the attribute from a `spread` stanza.

- `percent` `(integer:0)` - Specifies the percentage associated with the target value.

## `spread` Examples

The following examples show different ways to use the `spread` stanza.

### Even Spread Across Data Center

This example shows a spread stanza across the node's `datacenter` attribute. If we have
two datacenters `us-east1` and `us-west1`, and a task group of `count = 10`,
Nomad will attempt to place 5 allocations in each datacenter.

```hcl
spread {
  attribute = "${node.datacenter}"
  weight    = 100
}
```

### Spread With Target Percentages

This example shows a spread stanza that specifies one target percentage. If we
have three datacenters `us-east1`, `us-east2` and `us-west1`, and a task group
of `count = 10` Nomad will attempt to place place 5 of the allocations in "us-east1",
and then spread the rest among the other two datacenters.

```hcl
spread {
  attribute = "${node.datacenter}"
  weight    = 100

  target "us-east1" {
    percent = 50
  }
}
```

This example shows a spread stanza that specifies target percentages for two
different datacenters. If we have two datacenters `us-east1` and `us-west1`,
and a task group of `count = 10`, Nomad will attempt to place 6 allocations
in `us-east1` and 4 in `us-west1`.

```hcl
spread {
  attribute = "${node.datacenter}"
  weight    = 100

  target "us-east1" {
    percent = 60
  }

  target "us-west1" {
      percent = 40
  }
}
```

### Spread Across Multiple Attributes

This example shows spread stanzas with multiple attributes. Consider a Nomad cluster
where there are two datacenters `us-east1` and `us-west1`, and each datacenter has nodes
with `${meta.rack}` being `r1` or `r2`. For the following spread stanza used on a job with `count=12`, Nomad
will attempt to place 6 allocations in each datacenter. Within a datacenter, Nomad will
attempt to place 3 allocations in nodes on rack `r1`, and 3 allocations in nodes on rack `r2`.

```hcl
spread {
  attribute = "${node.datacenter}"
  weight    = 50
}
spread {
  attribute = "${meta.rack}"
  weight    = 50
}
```

[job]: /docs/job-specification/job.html "Nomad job Job Specification"
[group]: /docs/job-specification/group.html "Nomad group Job Specification"
[client-meta]: /docs/configuration/client.html#meta "Nomad meta Job Specification"
[task]: /docs/job-specification/task.html "Nomad task Job Specification"
[interpolation]: /docs/runtime/interpolation.html "Nomad interpolation"
[node-variables]: /docs/runtime/interpolation.html#node-variables- "Nomad interpolation-Node variables"
[constraint]: /docs/job-specification/constraint.html "Nomad Constraint job Specification"
