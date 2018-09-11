---
layout: "docs"
page_title: "parameterized Stanza - Job Specification"
sidebar_current: "docs-job-specification-parameterized"
description: |-
    A parameterized job is used to encapsulate a set of work that can be carried
    out on various inputs much like a function definition. When the
    `parameterized` stanza is added to a job, the job acts as a function to the
    cluster as a whole.
---

# `parameterized` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> **parameterized**</code>
    </td>
  </tr>
</table>

A parameterized job is used to encapsulate a set of work that can be carried out
on various inputs much like a function definition. When the `parameterized`
stanza is added to a job, the job acts as a function to the cluster as a whole.

The `parameterized` stanza allows job operators to configure a job that carries
out a particular action, define its resource requirements and configure how
inputs and configuration are retrieved by the tasks within the job.

To invoke a parameterized job, [`nomad job
dispatch`][dispatch command] or the equivalent HTTP APIs are
used. When dispatching against a parameterized job, an opaque payload and
metadata may be injected into the job. These inputs to the parameterized job act
like arguments to a function. The job consumes them to change it's behavior,
without exposing the implementation details to the caller.

To that end, tasks within the job can add a
[`dispatch_payload`][dispatch_payload] stanza that
defines where on the filesystem this payload gets written to. An example payload
would be a task's JSON configuration.

Further, certain metadata may be marked as required when dispatching a job so it
can be used to inject configuration directly into a task's arguments using
[interpolation]. An example of this would be to require a run ID key that
could be used to lookup the work the job is suppose to do from a management
service or database.

Each time a job is dispatched, a unique job ID is generated. This allows a
caller to track the status of the job, much like a future or promise in some
programming languages.

```hcl
job "docs" {
  parameterized {
    payload       = "required"
    meta_required = ["dispatcher_email"]
    meta_optional = ["pager_email"]
  }
}
```

## `parameterized` Requirements

 - The job's [scheduler type][batch-type] must be `batch`.

## `parameterized` Parameters

- `meta_optional` `(array<string>: nil)` - Specifies the set of metadata keys that
   may be provided when dispatching against the job.

- `meta_required` `(array<string>: nil)` - Specifies the set of metadata keys that
  must be provided when dispatching against the job.

- `payload` `(string: "optional")` - Specifies the requirement of providing a
  payload when dispatching against the parameterized job. The **maximum size of a
  `payload` is 16 KiB**. The options for this
  field are:

  - `"optional"` - A payload is optional when dispatching against the job.

  - `"required"` - A payload must be provided when dispatching against the job.

  - `"forbidden"` - A payload is forbidden when dispatching against the job.

## `parameterized` Examples

The following examples show non-runnable example parameterized jobs:

### Required Inputs

This example shows a parameterized job that requires both a payload and
metadata:

```hcl
job "video-encode" {
  # ...

  type = "batch"

  parameterized {
    payload       = "required"
    meta_required = ["dispatcher_email"]
  }

  group "encode" {
    # ...

    task "ffmpeg" {
      driver = "exec"

      config {
        command = "ffmpeg-wrapper"

        # When dispatched, the payload is written to a file that is then read by
        # the created task upon startup
        args = ["-config=${NOMAD_TASK_DIR}/config.json"]
      }

      dispatch_payload {
        file = "config.json"
      }
    }
  }
}
```

### Metadata Interpolation

```hcl
job "email-blast" {
  # ...

  type = "batch"

  parameterized {
    payload       = "forbidden"
    meta_required = ["CAMPAIGN_ID"]
  }

  group "emails" {
    # ...

    task "emailer" {
      driver = "exec"

      config {
        command = "emailer"

        # The campaign ID is interpolated and injected into the task's
        # arguments
        args = ["-campaign=${NOMAD_META_CAMPAIGN_ID}"]
      }
    }
  }
}
```

[batch-type]: /docs/job-specification/job.html#type "Batch scheduler type"
[dispatch command]: /docs/commands/job/dispatch.html "Nomad Job Dispatch Command"
[resources]: /docs/job-specification/resources.html "Nomad resources Job Specification"
[interpolation]: /docs/runtime/interpolation.html "Nomad Runtime Interpolation"
[dispatch_payload]: /docs/job-specification/dispatch_payload.html "Nomad dispatch_payload Job Specification"
