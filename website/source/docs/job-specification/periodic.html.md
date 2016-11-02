---
layout: "docs"
page_title: "periodic Stanza - Job Specification"
sidebar_current: "docs-job-specification-periodic"
description: |-
  The "periodic" stanza allows a job to run at fixed times, dates, or intervals.
  The easiest way to think about the periodic scheduler is "Nomad cron" or
  "distributed cron".
---

# `periodic` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> **periodic**</code>
    </td>
  </tr>
</table>

The `periodic` stanza allows a job to run at fixed times, dates, or intervals.
The easiest way to think about the periodic scheduler is "Nomad cron" or
"distributed cron".

```hcl
job "docs" {
  periodic {
    cron             = "*/15 * * * * *"
    prohibit_overlap = true
  }
}
```

The periodic expression is always evaluated in the **UTC timezone** to ensure
consistent evaluation when Nomad spans multiple time zones.

## `periodic` Parameters

- `cron` `(string: <required>)` - Specifies a cron expression configuring the
  interval to launch the job. In addition to [cron-specific formats][cron], this
  option also includes predefined expressions such as `@daily` or `@weekly`.

- `prohibit_overlap` `(bool: false)` - Specifies if this job should wait until
  previous instances of this job have completed. This only applies to this job;
  it does not prevent other periodic jobs from running at the same time.

## `periodic` Examples

The following examples only show the `periodic` stanzas. Remember that the
`periodic` stanza is only valid in the placements listed above.

### Run Daily

This example shows running a periodic job daily:

```hcl
periodic {
  cron = "@daily"
}
```

[cron]: https://github.com/gorhill/cronexpr#implementation "List of cron expressions"
